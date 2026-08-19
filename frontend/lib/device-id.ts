// Identificação do terminal (estação de trabalho).
//
// O rodapé de classificação exibe de qual terminal a sessão está aberta. Não
// existe cadastro de estações no sistema, então o identificador é gerado na
// primeira visita e guardado no localStorage: estável por navegador/máquina,
// opaco (não carrega nada do usuário) e suficiente para o agente reconhecer
// "esta é a minha estação" e para o suporte correlacionar relatos.
//
// É identificação, não autenticação: o valor vive no cliente e pode ser
// apagado ou forjado. Quando o device binding entrar, o vínculo confiável
// virá do servidor — este módulo é o lugar natural para receber o ID emitido
// por ele, mantendo a mesma chave.

const DEVICE_ID_KEY = "tevunah.device-id";

/** Formato exibido: 4 caracteres hex maiúsculos (TERM-A3F1). */
function generate(): string {
  const b = new Uint8Array(2);
  crypto.getRandomValues(b);
  return Array.from(b, (n) => n.toString(16).padStart(2, "0"))
    .join("")
    .toUpperCase();
}

function isValid(v: string | null): v is string {
  return !!v && /^[0-9A-F]{4}$/.test(v);
}

/**
 * Devolve o ID desta estação, gerando e persistindo na primeira chamada.
 * Fora do browser (SSR) devolve string vazia — o rodapé só monta no cliente.
 */
export function readDeviceId(): string {
  if (typeof window === "undefined") return "";
  let id: string | null = null;
  try {
    id = window.localStorage.getItem(DEVICE_ID_KEY);
  } catch {
    // Storage bloqueado (modo privado, política do navegador).
  }
  if (isValid(id)) return id;
  const fresh = generate();
  try {
    window.localStorage.setItem(DEVICE_ID_KEY, fresh);
  } catch {
    // Sem persistência o ID muda a cada carga — degradação aceitável, o
    // rodapé continua mostrando algo verdadeiro sobre a aba atual.
  }
  return fresh;
}

/** Rótulo pronto para exibição ("TERM-A3F1"), vazio se não houver ID. */
export function deviceLabel(id: string): string {
  return id ? `TERM-${id}` : "";
}

/** Header da requisição — nome casado com TerminalHeader no backend. */
export const TERMINAL_HEADER = "X-Terminal-Id";

/**
 * Header de terminal para anexar às requisições. É o mesmo rótulo exibido no
 * rodapé, para que o auditor consiga casar o que vê na tela com o registro em
 * audit_log.actor_terminal sem tradução. Objeto vazio fora do browser.
 */
export function terminalHeaders(): Record<string, string> {
  const label = deviceLabel(readDeviceId());
  return label ? { [TERMINAL_HEADER]: label } : {};
}

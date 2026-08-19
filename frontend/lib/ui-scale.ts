// Escala da interface.
//
// A folha de estilo é toda em px (217 declarações de font-size, nenhuma em
// rem), então nem a preferência de fonte do navegador nem a do sistema
// operacional têm efeito aqui — a única alavanca que sobrava para o agente
// era o zoom do navegador. Este módulo traz essa alavanca para dentro do
// produto: `zoom` no elemento raiz escala fonte, padding, borda e ícone na
// mesma proporção, que é o que mantém a densidade do layout intacta.
//
// O padrão é 125% porque foi a escala validada em tela FHD — abaixo disso o
// texto de tabela (11px) e os rótulos (9.5px) cansam em jornada longa.
//
// Compensação de viewport: `zoom` NÃO reescala vh/vw (eles resolvem contra o
// viewport não-zoomado e só então são escalados, estourando a tela). Por isso
// a escala também é publicada como a variável CSS --ui-scale, e as poucas
// medidas em vh/vw do globals.css dividem por ela.

export const UI_SCALES = [1, 1.15, 1.25, 1.5] as const;

export type UiScale = (typeof UI_SCALES)[number];

export const DEFAULT_UI_SCALE: UiScale = 1.25;

/** Rótulo de exibição ("125%"). */
export function uiScaleLabel(scale: UiScale): string {
  return `${Math.round(scale * 100)}%`;
}

export const UI_SCALE_KEY = "tevunah.ui-scale";

function isUiScale(v: unknown): v is UiScale {
  return UI_SCALES.some((s) => s === v);
}

/** Lê a escala salva. Valor ausente, corrompido ou fora da lista → padrão. */
export function readUiScale(): UiScale {
  if (typeof window === "undefined") return DEFAULT_UI_SCALE;
  const raw = Number(window.localStorage.getItem(UI_SCALE_KEY));
  return isUiScale(raw) ? raw : DEFAULT_UI_SCALE;
}

/** Aplica a escala no documento e persiste a escolha. */
export function applyUiScale(scale: UiScale): void {
  const root = document.documentElement;
  root.style.zoom = String(scale);
  root.style.setProperty("--ui-scale", String(scale));
  try {
    window.localStorage.setItem(UI_SCALE_KEY, String(scale));
  } catch {
    // Modo privado / storage cheio: a escala vale para a sessão corrente e
    // não persistir é degradação aceitável — não vale derrubar a tela.
  }
}

/**
 * Script aplicado no <head>, antes da primeira pintura. Sem ele a página
 * nasce em 100% e salta para a escala do agente depois da hidratação — um
 * flash visível em toda navegação. Fica aqui, e não solto no layout, para
 * não duplicar a chave nem o padrão em dois lugares.
 */
export const UI_SCALE_BOOTSTRAP = `
(function () {
  try {
    var allowed = [${UI_SCALES.join(",")}];
    var raw = Number(localStorage.getItem(${JSON.stringify(UI_SCALE_KEY)}));
    var s = allowed.indexOf(raw) >= 0 ? raw : ${DEFAULT_UI_SCALE};
    document.documentElement.style.zoom = String(s);
    document.documentElement.style.setProperty('--ui-scale', String(s));
  } catch (e) {
    document.documentElement.style.zoom = '${DEFAULT_UI_SCALE}';
  }
})();
`.trim();

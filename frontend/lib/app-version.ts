// Versão nova depois de um deploy, para a aba que já estava aberta (a página
// é única: sem recarregar, o código antigo continua rodando). Quem detecta é
// o VersionWatcher; a troca de tela chama reloadIfStale() — o momento em que
// recarregar não perde nada que o analista esteja digitando.

export const BUILD_ID = process.env.NEXT_PUBLIC_BUILD_ID ?? "";

let stale = false;
const listeners = new Set<(v: boolean) => void>();

export function isStale() {
  return stale;
}

export function markStale() {
  if (stale) return;
  stale = true;
  listeners.forEach((fn) => fn(true));
}

export function onStale(fn: (v: boolean) => void) {
  listeners.add(fn);
  return () => {
    listeners.delete(fn);
  };
}

/** Recarrega se há versão nova; devolve true quando vai recarregar. */
export function reloadIfStale() {
  if (!stale) return false;
  window.location.reload();
  return true;
}

/** Pergunta ao servidor qual versão ele serve agora. */
export async function checkVersion() {
  if (!BUILD_ID) return;
  try {
    const r = await fetch("/version", { cache: "no-store" });
    if (!r.ok) return;
    const { build } = (await r.json()) as { build?: string };
    if (build && build !== BUILD_ID) markStale();
  } catch {
    // sem rede: tenta de novo na próxima
  }
}

"use client";

import { useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";
import { checkVersion, isStale, onStale } from "@/lib/app-version";

// De quanto em quanto tempo a aba aberta confere a versão (além de quando o
// analista volta para ela).
const EVERY_MS = 2 * 60_000;
// Uma recarga por erro de arquivo antigo a cada este tempo, para não entrar
// em laço se o problema for outro.
const CHUNK_RELOAD_GUARD_MS = 60_000;

/**
 * Vigia a versão do sistema. Depois de um deploy, a aba que já estava aberta
 * recarrega sozinha na próxima troca de tela (Shell) e mostra a faixa com
 * ATUALIZAR AGORA. Se a aba pedir um arquivo da versão antiga que o deploy
 * apagou (ChunkLoadError), recarrega na hora.
 */
export default function VersionWatcher() {
  const [stale, setStale] = useState(isStale());

  useEffect(() => {
    const off = onStale(setStale);
    checkVersion();
    const t = window.setInterval(checkVersion, EVERY_MS);
    const onVisible = () => document.visibilityState === "visible" && checkVersion();
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("focus", onVisible);

    const reloadOnChunkError = (msg: string) => {
      if (!/ChunkLoadError|Loading chunk|Failed to fetch dynamically imported module|Importing a module script failed/i.test(msg)) return;
      const last = Number(sessionStorage.getItem("tevunah-chunk-reload") || 0);
      if (Date.now() - last < CHUNK_RELOAD_GUARD_MS) return;
      sessionStorage.setItem("tevunah-chunk-reload", String(Date.now()));
      window.location.reload();
    };
    const onError = (e: ErrorEvent) => reloadOnChunkError(String(e.message || e.error?.name || ""));
    const onRejection = (e: PromiseRejectionEvent) =>
      reloadOnChunkError(String((e.reason as Error)?.name || "") + " " + String((e.reason as Error)?.message || e.reason || ""));
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      off();
      window.clearInterval(t);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("focus", onVisible);
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);

  if (!stale) return null;
  return (
    <div className="version-banner" role="status">
      <RefreshCw size={12} /> NOVA VERSÃO DO SISTEMA — ATUALIZA SOZINHA NA PRÓXIMA TROCA DE TELA
      <button type="button" className="btn btn-sm btn-primary" onClick={() => window.location.reload()}>
        ATUALIZAR AGORA
      </button>
    </div>
  );
}

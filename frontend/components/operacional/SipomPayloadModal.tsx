"use client";

import { useEffect, useState } from "react";
import { Copy, Download, Image as ImageIcon, X } from "lucide-react";
import { getSipomPayload } from "@/lib/sipom-api";
import type { ApiError } from "@/lib/api";

// Na tela o base64 aparece abreviado: uma foto são dezenas de milhares de
// caracteres numa linha só. COPIAR e BAIXAR levam o JSON inteiro.
const B64_SHOWN = 48;

function abbreviate(_key: string, value: unknown) {
  if (_key !== "base64" || typeof value !== "string") return value;
  if (value === "") return "(conteúdo omitido — clique em INCLUIR FOTOS)";
  if (value.length <= B64_SHOWN) return value;
  const kb = Math.max(1, Math.round((value.length * 3) / 4 / 1024));
  return `${value.slice(0, B64_SHOWN)}… (abreviado na tela: ${kb} KB)`;
}

// Quantas fotos o envio leva (envolvidos[].foto e fotos[]).
function countPhotos(payload: unknown): number {
  const p = payload as { envolvidos?: { foto?: unknown }[]; fotos?: unknown[] } | null;
  return (p?.fotos?.length ?? 0) + (p?.envolvidos?.filter((e) => e.foto).length ?? 0);
}

/**
 * Prévia do envio ao SIPOM: o JSON exato do POST para esta ocorrência. Abre
 * sem o conteúdo das fotos (só o tipo); INCLUIR FOTOS busca o base64 — é o
 * corpo completo, como será enviado. Serve para conferir e para mostrar ao
 * desenvolvimento do SIPOM um caso real.
 */
export default function SipomPayloadModal({ occurrenceId, onClose }: { occurrenceId: string; onClose: () => void }) {
  const [payload, setPayload] = useState<unknown | null>(null);
  const [withPhotos, setWithPhotos] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [downloading, setDownloading] = useState(false);

  useEffect(() => {
    setLoading(true);
    getSipomPayload(occurrenceId, withPhotos)
      .then((r) => {
        if (!r.ready) {
          setError("Ocorrência com pendências: " + r.pendencias.map((p) => p.label).join("; "));
          return;
        }
        setPayload(r.payload);
      })
      .catch((e) => setError((e as ApiError).message || "Erro ao montar a prévia"))
      .finally(() => setLoading(false));
  }, [occurrenceId, withPhotos]);

  const photos = countPhotos(payload);
  const full = payload ? JSON.stringify(payload, null, 2) : "";
  const shown = payload ? JSON.stringify(payload, abbreviate, 2) : "";
  // Com foto no envio, o JSON só está completo depois de INCLUIR FOTOS.
  const complete = photos === 0 || withPhotos;

  async function copy() {
    try {
      await navigator.clipboard.writeText(full);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError("Não foi possível copiar");
    }
  }

  // O arquivo baixado é sempre o JSON completo, como será enviado: se a
  // prévia ainda está sem o conteúdo das fotos, busca-o antes de salvar.
  async function download() {
    let body = payload;
    if (!complete) {
      setDownloading(true);
      try {
        const r = await getSipomPayload(occurrenceId, true);
        body = r.payload;
      } catch (e) {
        setError((e as ApiError).message || "Erro ao montar o arquivo");
        return;
      } finally {
        setDownloading(false);
      }
    }
    if (!body) return;
    const ficha = (body as { ocorrencia?: { numero_ocorrencia?: string } }).ocorrencia?.numero_ocorrencia;
    const url = URL.createObjectURL(new Blob([JSON.stringify(body, null, 2)], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `sipom-ocorrencia-${ficha || occurrenceId}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal modal--wide sipom-payload" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div className="modal-hd">
          <span>PRÉVIA DO ENVIO AO SIPOM</span>
          <button type="button" className="action-btn" onClick={onClose} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>
        <div className="modal-bd">
          {error && <div className="banner banner-error">⚠ {error}</div>}
          {!error && !payload && <div className="muted">// MONTANDO…</div>}
          {payload != null && photos > 0 && (
            <div className={"banner " + (withPhotos ? "banner-info" : "banner-warn")}>
              {withPhotos
                ? `JSON COMPLETO, COM ${photos} FOTO${photos > 1 ? "S" : ""} EM BASE64. NA TELA O BASE64 APARECE ABREVIADO; COPIAR E BAIXAR LEVAM O CONTEÚDO INTEIRO.`
                : `ESTE ENVIO LEVA ${photos} FOTO${photos > 1 ? "S" : ""}. A PRÉVIA ABRE SEM O CONTEÚDO DELAS (BASE64 VAZIO). INCLUIR FOTOS MOSTRA O JSON COMO SERÁ ENVIADO; BAIXAR JSON JÁ SALVA O ARQUIVO COMPLETO.`}
            </div>
          )}
          {payload != null && <pre className="sipom-json">{shown}</pre>}
        </div>
        <div className="filters-modal-ft">
          {photos > 0 && !withPhotos && (
            <button type="button" className="btn" onClick={() => setWithPhotos(true)} disabled={loading}>
              <ImageIcon size={13} strokeWidth={1.8} /> {loading ? "BUSCANDO…" : "INCLUIR FOTOS"}
            </button>
          )}
          <div style={{ marginLeft: "auto" }} />
          <button type="button" className="btn" onClick={copy} disabled={!payload || loading} title={complete ? undefined : "Sem o conteúdo das fotos"}>
            <Copy size={13} strokeWidth={1.8} /> {copied ? "COPIADO" : complete ? "COPIAR JSON" : "COPIAR SEM FOTOS"}
          </button>
          <button
            type="button"
            className="btn btn-primary"
            onClick={download}
            disabled={!payload || loading || downloading}
            title="Salva o JSON completo, com as fotos em base64, como será enviado"
          >
            <Download size={13} strokeWidth={1.8} /> {downloading ? "MONTANDO…" : "BAIXAR JSON"}
          </button>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            FECHAR
          </button>
        </div>
      </div>
    </div>
  );
}

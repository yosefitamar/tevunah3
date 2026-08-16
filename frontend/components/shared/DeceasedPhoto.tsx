"use client";

import { useEffect, useState } from "react";
import { X } from "lucide-react";

type Props = {
  src: string;
  alt?: string;
  /** Quando false, renderiza a foto normal (sem P&B nem tarja). */
  deceased: boolean;
  /**
   * Classe do elemento externo. Vai no wrapper (não no <img>) porque alguns
   * contextos posicionam a foto — o preview em hover da lista de envolvidos,
   * por exemplo, é absoluto — e a tarja precisa acompanhar esse mesmo box.
   */
  className?: string;
  /** Texto da tarja. */
  label?: string;
  /** Tarja menor, para miniaturas em lista. */
  compact?: boolean;
};

/**
 * Foto de pessoa marcada como óbito: dessaturada, com tarja vermelha.
 *
 * O tratamento é só visual — o arquivo original nunca é alterado. Clicar
 * abre a foto como está no acervo (colorida, sem tarja), porque a
 * identificação visual do rosto é justamente o que o agente precisa.
 */
export default function DeceasedPhoto({
  src,
  alt = "",
  deceased,
  className,
  label = "ÓBITO",
  compact = false,
}: Props) {
  const [showOriginal, setShowOriginal] = useState(false);
  const [failed, setFailed] = useState(false);

  // Escape fecha a foto ampliada.
  useEffect(() => {
    if (!showOriginal) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setShowOriginal(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [showOriginal]);

  if (failed) return null;

  if (!deceased) {
    return (
      <span className={"photo-frame " + (className ?? "")}>
        <img src={src} alt={alt} onError={() => setFailed(true)} />
      </span>
    );
  }

  return (
    <>
      <button
        type="button"
        className={"photo-frame deceased-photo " + (className ?? "")}
        onClick={(e) => {
          e.stopPropagation();
          setShowOriginal(true);
        }}
        title="Clique para ver a foto original"
        aria-label={`${label} — ver foto original`}
      >
        <img src={src} alt={alt} onError={() => setFailed(true)} />
        <span className={"deceased-band" + (compact ? " deceased-band--compact" : "")}>
          {label}
        </span>
      </button>

      {showOriginal && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            e.stopPropagation();
            setShowOriginal(false);
          }}
        >
          <div className="deceased-original" onClick={(e) => e.stopPropagation()}>
            <div className="modal-hd">
              <span>FOTO ORIGINAL</span>
              <button
                type="button"
                className="action-btn"
                onClick={() => setShowOriginal(false)}
                aria-label="Fechar"
              >
                <X size={14} />
              </button>
            </div>
            <img src={src} alt={alt} />
          </div>
        </div>
      )}
    </>
  );
}

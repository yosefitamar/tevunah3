"use client";

import { useEffect, useState } from "react";
import { AlertTriangle, X } from "lucide-react";
import { listEntities, photoURL } from "@/lib/entities-api";
import type { PersonAttrs } from "@/lib/entities-types";
import { formatBRDate } from "@/lib/format";

export type VictimCandidate = {
  id: string;
  name: string;
  kind: string;
  version: number;
  attrs?: PersonAttrs;
};

type Props = {
  candidate: VictimCandidate;
  /** Data da ocorrência (YYYY-MM-DD), que vira a data do óbito. */
  occurredOn: string;
  onConfirm: () => void;
  onCancel: () => void;
};

/**
 * Confirmação antes de vincular alguém como VÍTIMA de homicídio.
 *
 * Vincular marca a pessoa como óbito no acervo inteiro — um erro de
 * homônimo mata a pessoa errada no sistema. Por isso a tela mostra os dados
 * que distinguem homônimos (CPF, nome da mãe, nascimento, foto) e avisa
 * quantas outras pessoas compartilham exatamente o mesmo nome.
 */
export default function ConfirmVitimaModal({
  candidate,
  occurredOn,
  onConfirm,
  onCancel,
}: Props) {
  const a = candidate.attrs ?? {};
  const [namesakes, setNamesakes] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    listEntities({ search: candidate.name, limit: 50 })
      .then((r) => {
        if (cancelled) return;
        const same = (r.items || []).filter(
          (e) =>
            e.id !== candidate.id &&
            e.kind === "person" &&
            e.name.trim().toUpperCase() === candidate.name.trim().toUpperCase(),
        );
        setNamesakes(same.length);
      })
      .catch(() => setNamesakes(null));
    return () => {
      cancelled = true;
    };
  }, [candidate.id, candidate.name]);

  const alias = a.aliases && a.aliases.length > 0 ? a.aliases[0] : "";

  return (
    <div className="modal-backdrop" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 520 }}>
        <div className="modal-hd">
          <span>CONFIRMAR VÍTIMA</span>
          <button type="button" className="action-btn" onClick={onCancel} aria-label="Fechar">
            <X size={14} />
          </button>
        </div>

        <div className="modal-bd">
          {/* O texto vai num único nó: .banner é flex, e quebrá-lo em
              vários filhos espalharia as frases pelas colunas. */}
          <div className="banner banner-warn">
            <AlertTriangle size={13} strokeWidth={1.8} />
            <span>
              Confirmar vincula esta pessoa como vítima e a marca como ÓBITO em todo o
              sistema. Verifique que não é homônimo.
            </span>
          </div>

          <div className="victim-confirm">
            {a.has_photo && (
              <img
                className="victim-confirm-photo"
                src={photoURL(candidate.id, candidate.version)}
                alt=""
                aria-hidden
              />
            )}
            <dl className="dossier-list victim-confirm-data">
              <div>
                <dt>NOME</dt>
                <dd>
                  {candidate.name.toUpperCase()}
                  {alias ? ` (${alias.toUpperCase()})` : ""}
                </dd>
              </div>
              <div>
                <dt>CPF</dt>
                <dd>{a.cpf || "—"}</dd>
              </div>
              <div>
                <dt>NASCIMENTO</dt>
                <dd>{a.date_of_birth ? formatBRDate(a.date_of_birth) : "—"}</dd>
              </div>
              <div>
                <dt>NOME DA MÃE</dt>
                <dd>{a.mother_name || "—"}</dd>
              </div>
              <div>
                <dt>DATA DO ÓBITO</dt>
                <dd>{formatBRDate(occurredOn)}</dd>
              </div>
            </dl>
          </div>

          {namesakes != null && namesakes > 0 && (
            <div className="banner banner-error">
              ⚠ Há {namesakes} outra{namesakes > 1 ? "s" : ""} pessoa
              {namesakes > 1 ? "s" : ""} com este mesmo nome no acervo. Confirme pelo CPF,
              nome da mãe ou foto antes de prosseguir.
            </div>
          )}
          {!a.cpf && !a.mother_name && !a.date_of_birth && (
            <div className="muted" style={{ fontSize: 10.5 }}>
              // ESTE CADASTRO NÃO TEM CPF, NOME DA MÃE NEM DATA DE NASCIMENTO — A
              CONFERÊNCIA DEPENDE DA FOTO OU DE OUTRA FONTE
            </div>
          )}
        </div>

        <div className="modal-ft">
          <button type="button" className="btn btn-ghost" onClick={onCancel}>
            CANCELAR
          </button>
          <button type="button" className="btn btn-danger" onClick={onConfirm}>
            CONFIRMAR VÍTIMA E MARCAR ÓBITO
          </button>
        </div>
      </div>
    </div>
  );
}

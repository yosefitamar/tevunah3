"use client";

import { REPORT_STATUS_LABEL, type ReportStatus } from "@/lib/reports-api";
import { ENTITY_KIND_LABEL, type EntityKind } from "@/lib/entities-types";
import type { DashEntities, DashInformes, DashReports } from "@/lib/dashboard-api";
import FacetBars, { type FacetBar } from "./FacetBars";

// Cor por status do RI: o difundido é o que cumpriu o ciclo (accent), o
// rascunho é trabalho em aberto (warn) e o arquivado saiu de circulação
// (cinza). Mesma leitura das pills da listagem de relatórios.
const STATUS_COLOR: Record<ReportStatus, string> = {
  criado: "var(--warn)",
  difundido: "var(--accent)",
  arquivado: "var(--fg-3)",
};

const STATUS_ORDER: ReportStatus[] = ["criado", "difundido", "arquivado"];
const KIND_ORDER: EntityKind[] = ["person", "organization", "vehicle", "place"];

// ENTITY_KIND_LABEL é singular ("PESSOA"); o acervo fala de conjuntos.
const KIND_PLURAL: Record<EntityKind, string> = {
  person: "PESSOAS",
  organization: "ORGANIZAÇÕES",
  place: "LUGARES",
  vehicle: "VEÍCULOS",
};

type Props = {
  reports?: DashReports;
  informes?: DashInformes;
  entities?: DashEntities;
  /** Falso quando o recorte é aberto: sem base, não se exibe variação. */
  hasBaseline: boolean;
};

// delta descreve a variação em números absolutos. Aqui não se usa percentual:
// informe e entidade saem em dezenas por mês, faixa em que a variação
// relativa oscila de forma violenta e comunica menos que "9 contra 7".
function delta(current: number, previous: number, hasBaseline: boolean): string {
  if (!hasBaseline) return "";
  const d = current - previous;
  if (d === 0) return ` · estável (${previous})`;
  return ` · ${d > 0 ? "+" : "−"}${Math.abs(d)} vs. ${previous}`;
}

/**
 * Produção da agência — o que a inteligência entregou no recorte, ao lado do
 * acervo acumulado. Complementa os KPIs de criminalidade: aqueles medem o que
 * aconteceu no território, este mede o que a casa produziu sobre isso.
 */
export default function ProductionPanel({ reports, informes, entities, hasBaseline }: Props) {
  const statusBars: FacetBar[] = reports
    ? STATUS_ORDER.map((s) => ({
        key: s,
        label: REPORT_STATUS_LABEL[s],
        count: reports.by_status[s] ?? 0,
        color: STATUS_COLOR[s],
      }))
    : [];

  return (
    <div className="panel">
      <div className="panel-hd">
        <span className="ttl">PRODUÇÃO DA AGÊNCIA</span>
        <span className="meta">ACERVO E PERÍODO</span>
      </div>
      <div className="panel-bd">
        {reports && (
          <>
            <div className="prod-sub">RELATÓRIOS DE INTELIGÊNCIA · ACERVO</div>
            <FacetBars items={statusBars} empty="NENHUM RI VISÍVEL" />
          </>
        )}

        <dl className="dossier-list prod-list" style={{ marginTop: reports ? 14 : 0 }}>
          {reports && (
            <>
              <div>
                <dt>RIs DO PERÍODO</dt>
                <dd>
                  {reports.created}
                  <span className="muted"> · por data do documento</span>
                </dd>
              </div>
              <div>
                <dt>DIFUNDIDOS</dt>
                <dd>
                  {reports.diffused}
                  <span className="muted">
                    {delta(reports.diffused, reports.prev_diffused, hasBaseline)}
                  </span>
                </dd>
              </div>
            </>
          )}
          {informes && (
            <div>
              <dt>INFORMES</dt>
              <dd>
                {informes.created}
                <span className="muted">
                  {delta(informes.created, informes.prev, hasBaseline)} · acervo {informes.total}
                </span>
              </dd>
            </div>
          )}
          {entities && (
            <>
              <div>
                <dt>ENTIDADES NOVAS</dt>
                <dd>
                  {entities.created}
                  <span className="muted">{delta(entities.created, entities.prev, hasBaseline)}</span>
                </dd>
              </div>
              <div>
                <dt>ACERVO</dt>
                <dd>
                  {KIND_ORDER.filter((k) => (entities.by_kind[k] ?? 0) > 0)
                    .map(
                      (k) =>
                        `${entities.by_kind[k]} ${
                          entities.by_kind[k] === 1 ? ENTITY_KIND_LABEL[k] : KIND_PLURAL[k]
                        }`,
                    )
                    .join(" · ") || "—"}
                </dd>
              </div>
              <div>
                <dt>ÓBITOS REGISTRADOS</dt>
                <dd>{entities.deceased}</dd>
              </div>
            </>
          )}
        </dl>

        {!reports && !informes && !entities && (
          <div className="muted" style={{ fontSize: 11 }}>
            // SEM PERMISSÃO DE LEITURA DOS MÓDULOS DE PRODUÇÃO
          </div>
        )}
      </div>
    </div>
  );
}

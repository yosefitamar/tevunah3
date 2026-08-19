"use client";

export type FacetBar = {
  key: string;
  label: string;
  /** Linha secundária do rótulo (ex.: o município de um bairro). */
  sub?: string;
  count: number;
  /** Cor da barra (token da paleta); default = accent. */
  color?: string;
};

type Props = {
  items: FacetBar[];
  /** Texto do vazio, no dialeto do módulo. */
  empty: string;
  /** Mostra a fatia percentual ao lado da contagem. */
  showPercent?: boolean;
};

/**
 * Ranking horizontal — a leitura de "onde se concentra" que a agência já faz
 * no painel lateral do mapa do crime.
 *
 * As barras são proporcionais ao MAIOR item da lista, não ao total: com oito
 * territórios disputando um período curto, a escala pelo total achataria
 * todas contra a margem esquerda e a comparação entre elas, que é o ponto,
 * desapareceria. O percentual (quando pedido) continua sendo sobre o total,
 * porque ali a pergunta é outra — quanto do conjunto aquilo representa.
 */
export default function FacetBars({ items, empty, showPercent = false }: Props) {
  if (items.length === 0) {
    return (
      <div className="muted" style={{ fontSize: 11 }}>
        // {empty}
      </div>
    );
  }
  const max = Math.max(...items.map((i) => i.count), 1);
  const total = items.reduce((acc, i) => acc + i.count, 0);

  return (
    <div className="bars">
      {items.map((i) => (
        <div key={i.key} className="bar bar--facet">
          <div className="lbl" title={i.sub ? `${i.label} · ${i.sub}` : i.label}>
            {i.label}
            {i.sub && <span className="sub">{i.sub}</span>}
          </div>
          <div className="track">
            <div
              className="fill"
              // A cor entra como custom property porque o risco luminoso da
              // ponta da barra (::after) precisa da mesma cor, e pseudo-
              // elemento não alcança style inline.
              style={
                {
                  // Piso de 2% para o valor pequeno não sumir — mas zero não
                  // ganha traço: barra visível onde não houve ocorrência mente
                  // sobre o dado.
                  width: i.count === 0 ? "0" : `${Math.max((i.count / max) * 100, 2)}%`,
                  ...(i.color ? { ["--bar-color"]: i.color } : null),
                } as React.CSSProperties
              }
            />
          </div>
          <div className="val">
            {i.count}
            {showPercent && total > 0 && (
              <span className="muted"> · {Math.round((i.count / total) * 100)}%</span>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

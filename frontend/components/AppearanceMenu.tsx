"use client";

import { PALETTES, type PaletteId } from "@/lib/palettes";
import {
  UI_SCALES,
  applyUiScale,
  uiScaleLabel,
  type UiScale,
} from "@/lib/ui-scale";

type Props = {
  palette: PaletteId;
  setPalette: (p: PaletteId) => void;
  scale: UiScale;
  setScale: (s: UiScale) => void;
};

/**
 * Popover da engrenagem: preferências visuais do posto de trabalho.
 *
 * A escala existe porque a folha é toda em px — a preferência de fonte do
 * navegador e a do sistema não têm efeito no Tevunah, então sem este
 * controle a única saída do agente era o zoom do navegador. Fica salva no
 * navegador (localStorage), não no usuário: é preferência de terminal, e
 * quem senta na mesma máquina costuma querer a mesma escala.
 */
export default function AppearanceMenu({
  palette,
  setPalette,
  scale,
  setScale,
}: Props) {
  return (
    <div className="appearance-menu" role="group" aria-label="Aparência">
      <div className="appearance-group">
        <span className="appearance-lbl">ESCALA</span>
        <div className="appearance-row">
          {UI_SCALES.map((s) => (
            <button
              key={s}
              type="button"
              className={"appearance-opt" + (scale === s ? " on" : "")}
              aria-pressed={scale === s}
              onClick={() => {
                applyUiScale(s);
                setScale(s);
              }}
              title={`Escala da interface em ${uiScaleLabel(s)}`}
            >
              {uiScaleLabel(s)}
            </button>
          ))}
        </div>
      </div>

      <div className="appearance-group">
        <span className="appearance-lbl">PALETA</span>
        <div className="appearance-row">
          {(Object.keys(PALETTES) as PaletteId[]).map((k) => {
            const p = PALETTES[k];
            return (
              <button
                key={k}
                type="button"
                className={"appearance-opt appearance-opt--swatch" + (palette === k ? " on" : "")}
                onClick={() => setPalette(k)}
                aria-pressed={palette === k}
                title={p.lbl}
              >
                <span
                  className="sw"
                  style={{
                    background: p.swatch,
                    boxShadow: palette === k ? `0 0 8px ${p.swatch}` : "none",
                  }}
                />
                {p.lbl}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

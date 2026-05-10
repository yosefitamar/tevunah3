"use client";

import { PALETTES, type PaletteId } from "@/lib/palettes";

type Props = {
  palette: PaletteId;
  setPalette: (p: PaletteId) => void;
};

export default function PaletteSwitcher({ palette, setPalette }: Props) {
  return (
    <div className="palette-switcher" role="group" aria-label="Trocar paleta">
      {(Object.keys(PALETTES) as PaletteId[]).map((k) => {
        const p = PALETTES[k];
        return (
          <button
            key={k}
            className={palette === k ? "on" : ""}
            onClick={() => setPalette(k)}
            type="button"
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
  );
}

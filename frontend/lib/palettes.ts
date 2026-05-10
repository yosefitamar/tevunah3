export type PaletteId = "phosphor" | "amber" | "cyan" | "alert";

export const PALETTES: Record<PaletteId, { lbl: string; swatch: string }> = {
  phosphor: { lbl: "FÓSFORO", swatch: "#00ff7a" },
  amber: { lbl: "ÂMBAR", swatch: "#ffb000" },
  cyan: { lbl: "CIANO", swatch: "#4dd0ff" },
  alert: { lbl: "ALERTA", swatch: "#ff3b3b" },
};

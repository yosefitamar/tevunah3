import { useState, type DragEvent } from "react";

export type FileDropHandlers = {
  onDragEnter: (e: DragEvent) => void;
  onDragOver: (e: DragEvent) => void;
  onDragLeave: (e: DragEvent) => void;
  onDrop: (e: DragEvent) => void;
};

// useFileDrop fornece handlers de arrastar-e-soltar para uma área. Ao soltar
// um arquivo, chama onFile com o primeiro arquivo. Combine com um
// <input type="file"> (clique) para que o campo aceite as duas formas — soltar
// a imagem OU escolher o arquivo. `disabled` ignora o drop.
export function useFileDrop(
  onFile: (f: File | null) => void,
  disabled = false,
): { dragging: boolean; handlers: FileDropHandlers } {
  const [dragging, setDragging] = useState(false);

  const over = (e: DragEvent) => {
    e.preventDefault();
    if (disabled) return;
    e.dataTransfer.dropEffect = "copy";
    if (!dragging) setDragging(true);
  };

  return {
    dragging,
    handlers: {
      onDragEnter: over,
      onDragOver: over,
      onDragLeave: (e) => {
        e.preventDefault();
        setDragging(false);
      },
      onDrop: (e) => {
        e.preventDefault();
        setDragging(false);
        if (disabled) return;
        onFile(e.dataTransfer.files?.[0] ?? null);
      },
    },
  };
}

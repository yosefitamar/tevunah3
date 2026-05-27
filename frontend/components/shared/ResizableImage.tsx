"use client";

import { useRef } from "react";
import { Image as TiptapImage } from "@tiptap/extension-image";
import {
  NodeViewWrapper,
  ReactNodeViewRenderer,
  type NodeViewProps,
} from "@tiptap/react";

// ResizableImage estende o Image oficial adicionando o atributo `width`,
// serializado como inline style (`width: NNNpx`). Um NodeView React
// envolve o <img> e expõe uma alça de redimensionamento no canto inferior
// direito quando a imagem está selecionada ou sob hover.
//
// Mantém o mesmo nome de nó ("image") do Image padrão, então o comando
// editor.commands.setImage({ src, alt }) e o parsing de HTML existente
// continuam funcionando.
export const ResizableImage = TiptapImage.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      width: {
        default: null,
        parseHTML: (el) => {
          const html = el as HTMLElement;
          const styleW = html.style?.width;
          if (styleW) return styleW;
          const attrW = html.getAttribute("width");
          if (!attrW) return null;
          return /^\d+$/.test(attrW) ? `${attrW}px` : attrW;
        },
        renderHTML: (attrs: { width?: string | null }) => {
          if (!attrs.width) return {};
          return { style: `width: ${attrs.width}` };
        },
      },
    };
  },
  addNodeView() {
    return ReactNodeViewRenderer(ImageView);
  },
});

function ImageView({ node, updateAttributes, selected, editor }: NodeViewProps) {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const editable = editor.isEditable;

  const width = (node.attrs as { width?: string | null }).width || null;
  const src = node.attrs.src as string;
  const alt = (node.attrs.alt as string | null) ?? "";

  function startResize(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    const img = wrapperRef.current?.querySelector("img");
    if (!img) return;
    const startX = e.clientX;
    const startWidth = img.getBoundingClientRect().width;
    const max =
      wrapperRef.current?.parentElement?.getBoundingClientRect().width ?? 9999;

    function onMove(ev: MouseEvent) {
      const delta = ev.clientX - startX;
      const next = Math.max(60, Math.min(max, startWidth + delta));
      updateAttributes({ width: `${Math.round(next)}px` });
    }
    function onUp() {
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    }
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }

  return (
    <NodeViewWrapper
      as="div"
      ref={wrapperRef}
      className={"rte-img" + (selected ? " rte-img--selected" : "")}
      data-drag-handle
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt={alt}
        style={width ? { width } : undefined}
        draggable={false}
      />
      {editable && (
        <span
          className="rte-img-handle"
          onMouseDown={startResize}
          role="button"
          aria-label="Redimensionar imagem"
        />
      )}
    </NodeViewWrapper>
  );
}

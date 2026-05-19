"use client";

import { useEffect } from "react";
import { useEditor, EditorContent, type Editor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Underline from "@tiptap/extension-underline";
import TextAlign from "@tiptap/extension-text-align";
import Link from "@tiptap/extension-link";
import Image from "@tiptap/extension-image";
import { Table } from "@tiptap/extension-table";
import TableRow from "@tiptap/extension-table-row";
import TableCell from "@tiptap/extension-table-cell";
import TableHeader from "@tiptap/extension-table-header";
import {
  AlignCenter,
  AlignJustify,
  AlignLeft,
  AlignRight,
  Bold,
  Heading2,
  Heading3,
  Image as ImageIcon,
  Italic,
  Link as LinkIcon,
  List,
  ListOrdered,
  Quote,
  Redo,
  Strikethrough,
  Table as TableIcon,
  Underline as UnderlineIcon,
  Undo,
} from "lucide-react";

type Props = {
  value: string;
  onChange: (html: string) => void;
  /** Read-only quando o relatório já foi difundido/arquivado. */
  disabled?: boolean;
  /** Callback opcional pra upload de imagem. Recebe File, devolve URL
   *  acessível pelo Gotenberg. Quando ausente, botão de imagem fica oculto. */
  onUploadImage?: (file: File) => Promise<string>;
};

/**
 * Editor rich-text baseado em TipTap. Estilo Tevunah (terminal/tactical):
 * toolbar compacta, monoespaçada nos rótulos, accent verde nos ativos.
 *
 * Saída: HTML serializado em `value` via prop `onChange`. O HTML é
 * compatível com o template do PDF (Gotenberg renderiza HTML via Chromium).
 */
export default function RichTextEditor({
  value,
  onChange,
  disabled,
  onUploadImage,
}: Props) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        // Justificado é o default do template — mas alinhamento é controlado
        // pelo TextAlign abaixo, não pelos defaults do StarterKit.
        heading: { levels: [2, 3] },
      }),
      Underline,
      TextAlign.configure({
        types: ["heading", "paragraph"],
        defaultAlignment: "justify",
      }),
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { rel: "noopener noreferrer" },
      }),
      Image.configure({
        inline: false,
        allowBase64: false,
      }),
      Table.configure({ resizable: true }),
      TableRow,
      TableHeader,
      TableCell,
    ],
    content: value || "<p></p>",
    editable: !disabled,
    immediatelyRender: false,
    onUpdate({ editor }) {
      onChange(editor.getHTML());
    },
  });

  // Sincroniza valor externo (ex.: trocou de relatório no mesmo drawer).
  useEffect(() => {
    if (!editor) return;
    if (editor.getHTML() === value) return;
    editor.commands.setContent(value || "<p></p>", { emitUpdate: false });
  }, [editor, value]);

  // Reflete prop disabled em runtime.
  useEffect(() => {
    if (!editor) return;
    editor.setEditable(!disabled);
  }, [editor, disabled]);

  if (!editor) {
    return (
      <div className="rte-wrap">
        <div className="muted" style={{ padding: 12, fontSize: 11 }}>
          // CARREGANDO EDITOR…
        </div>
      </div>
    );
  }

  return (
    <div className={"rte-wrap" + (disabled ? " rte-wrap--disabled" : "")}>
      <Toolbar editor={editor} onUploadImage={onUploadImage} disabled={disabled} />
      <EditorContent editor={editor} className="rte-content" />
    </div>
  );
}

function Toolbar({
  editor,
  onUploadImage,
  disabled,
}: {
  editor: Editor;
  onUploadImage?: (file: File) => Promise<string>;
  disabled?: boolean;
}) {
  function addLink() {
    const prev = editor.getAttributes("link").href as string | undefined;
    const url = window.prompt("URL do link:", prev ?? "https://");
    if (url === null) return;
    if (url === "") {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
      return;
    }
    editor.chain().focus().extendMarkRange("link").setLink({ href: url }).run();
  }

  function addImage() {
    if (!onUploadImage) return;
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.onchange = async () => {
      const f = input.files?.[0];
      if (!f) return;
      try {
        const url = await onUploadImage(f);
        editor.chain().focus().setImage({ src: url, alt: f.name }).run();
      } catch (e) {
        // eslint-disable-next-line no-alert
        alert("Falha ao subir imagem: " + (e instanceof Error ? e.message : "erro"));
      }
    };
    input.click();
  }

  function addTable() {
    editor
      .chain()
      .focus()
      .insertTable({ rows: 3, cols: 3, withHeaderRow: true })
      .run();
  }

  const isInTable = editor.isActive("table");

  return (
    <div className="rte-toolbar" role="toolbar" aria-disabled={disabled}>
      <Btn
        title="Desfazer (⌘Z)"
        onClick={() => editor.chain().focus().undo().run()}
        disabled={disabled || !editor.can().undo()}
      >
        <Undo size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Refazer (⌘⇧Z)"
        onClick={() => editor.chain().focus().redo().run()}
        disabled={disabled || !editor.can().redo()}
      >
        <Redo size={13} strokeWidth={1.8} />
      </Btn>
      <Sep />
      <Btn
        title="Negrito"
        active={editor.isActive("bold")}
        onClick={() => editor.chain().focus().toggleBold().run()}
        disabled={disabled}
      >
        <Bold size={13} strokeWidth={2} />
      </Btn>
      <Btn
        title="Itálico"
        active={editor.isActive("italic")}
        onClick={() => editor.chain().focus().toggleItalic().run()}
        disabled={disabled}
      >
        <Italic size={13} strokeWidth={2} />
      </Btn>
      <Btn
        title="Sublinhado"
        active={editor.isActive("underline")}
        onClick={() => editor.chain().focus().toggleUnderline().run()}
        disabled={disabled}
      >
        <UnderlineIcon size={13} strokeWidth={2} />
      </Btn>
      <Btn
        title="Tachado"
        active={editor.isActive("strike")}
        onClick={() => editor.chain().focus().toggleStrike().run()}
        disabled={disabled}
      >
        <Strikethrough size={13} strokeWidth={2} />
      </Btn>
      <Sep />
      <Btn
        title="Título 2"
        active={editor.isActive("heading", { level: 2 })}
        onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
        disabled={disabled}
      >
        <Heading2 size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Título 3"
        active={editor.isActive("heading", { level: 3 })}
        onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
        disabled={disabled}
      >
        <Heading3 size={13} strokeWidth={1.8} />
      </Btn>
      <Sep />
      <Btn
        title="Lista"
        active={editor.isActive("bulletList")}
        onClick={() => editor.chain().focus().toggleBulletList().run()}
        disabled={disabled}
      >
        <List size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Lista numerada"
        active={editor.isActive("orderedList")}
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
        disabled={disabled}
      >
        <ListOrdered size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Citação"
        active={editor.isActive("blockquote")}
        onClick={() => editor.chain().focus().toggleBlockquote().run()}
        disabled={disabled}
      >
        <Quote size={13} strokeWidth={1.8} />
      </Btn>
      <Sep />
      <Btn
        title="Alinhar à esquerda"
        active={editor.isActive({ textAlign: "left" })}
        onClick={() => editor.chain().focus().setTextAlign("left").run()}
        disabled={disabled}
      >
        <AlignLeft size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Centralizar"
        active={editor.isActive({ textAlign: "center" })}
        onClick={() => editor.chain().focus().setTextAlign("center").run()}
        disabled={disabled}
      >
        <AlignCenter size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Alinhar à direita"
        active={editor.isActive({ textAlign: "right" })}
        onClick={() => editor.chain().focus().setTextAlign("right").run()}
        disabled={disabled}
      >
        <AlignRight size={13} strokeWidth={1.8} />
      </Btn>
      <Btn
        title="Justificar"
        active={editor.isActive({ textAlign: "justify" })}
        onClick={() => editor.chain().focus().setTextAlign("justify").run()}
        disabled={disabled}
      >
        <AlignJustify size={13} strokeWidth={1.8} />
      </Btn>
      <Sep />
      <Btn title="Link" active={editor.isActive("link")} onClick={addLink} disabled={disabled}>
        <LinkIcon size={13} strokeWidth={1.8} />
      </Btn>
      {onUploadImage && (
        <Btn title="Inserir imagem" onClick={addImage} disabled={disabled}>
          <ImageIcon size={13} strokeWidth={1.8} />
        </Btn>
      )}
      <Btn
        title={isInTable ? "Remover tabela" : "Inserir tabela 3×3"}
        active={isInTable}
        onClick={() =>
          isInTable
            ? editor.chain().focus().deleteTable().run()
            : addTable()
        }
        disabled={disabled}
      >
        <TableIcon size={13} strokeWidth={1.8} />
      </Btn>
      {isInTable && (
        <>
          <Sep />
          <Btn
            title="Adicionar linha acima"
            onClick={() => editor.chain().focus().addRowBefore().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>+R↑</span>
          </Btn>
          <Btn
            title="Adicionar linha abaixo"
            onClick={() => editor.chain().focus().addRowAfter().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>+R↓</span>
          </Btn>
          <Btn
            title="Adicionar coluna à esquerda"
            onClick={() => editor.chain().focus().addColumnBefore().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>+C←</span>
          </Btn>
          <Btn
            title="Adicionar coluna à direita"
            onClick={() => editor.chain().focus().addColumnAfter().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>+C→</span>
          </Btn>
          <Btn
            title="Remover linha"
            onClick={() => editor.chain().focus().deleteRow().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>−R</span>
          </Btn>
          <Btn
            title="Remover coluna"
            onClick={() => editor.chain().focus().deleteColumn().run()}
            disabled={disabled}
          >
            <span style={{ fontSize: 9, letterSpacing: 0 }}>−C</span>
          </Btn>
        </>
      )}
    </div>
  );
}

function Btn({
  title,
  onClick,
  active,
  disabled,
  children,
}: {
  title: string;
  onClick: () => void;
  active?: boolean;
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className={"rte-btn" + (active ? " rte-btn--on" : "")}
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-pressed={active}
    >
      {children}
    </button>
  );
}

function Sep() {
  return <span className="rte-sep" aria-hidden />;
}

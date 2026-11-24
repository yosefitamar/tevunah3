"use client";

import { useRef, useState } from "react";
import { FileText } from "lucide-react";
import { parsePersonPdf, type PersonPdfResult } from "@/lib/entities-api";
import { useFileDrop } from "@/lib/useFileDrop";
import { formatBR } from "@/lib/format";
import type { ApiError } from "@/lib/api";

/**
 * Área para anexar um PDF de consulta (RELATÓRIO DE CPF ou CONSULTA
 * INTEGRADA) no cadastro de pessoa. O PDF vai ao backend, que extrai os dados
 * e descarta o arquivo; quem chama recebe o resultado em `onParsed` e
 * preenche o formulário. Nada é gravado aqui.
 */
export default function PersonPdfDrop({
  onParsed,
  disabled,
}: {
  onParsed: (r: PersonPdfResult) => void;
  disabled?: boolean;
}) {
  const fileRef = useRef<HTMLInputElement | null>(null);
  const [parsing, setParsing] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [loaded, setLoaded] = useState<
    { file: string; source: string; records: number; generatedAt?: string }[]
  >([]);

  const busy = parsing || !!disabled;
  const { dragging, handlers } = useFileDrop((f) => f && read(f), busy);

  async function read(file: File) {
    setErr(null);
    setParsing(true);
    try {
      const r = await parsePersonPdf(file);
      setLoaded((prev) => [
        ...prev,
        {
          file: file.name,
          source: r.report.source,
          records: r.report.criminal.length,
          generatedAt: r.report.generated_at,
        },
      ]);
      onParsed(r);
    } catch (e) {
      setErr((e as ApiError).message || "Erro ao ler o PDF");
    } finally {
      setParsing(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <div className="form-field">
      <span>PDF DE CONSULTA (OPCIONAL)</span>
      <input
        ref={fileRef}
        type="file"
        accept="application/pdf"
        style={{ display: "none" }}
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) read(f);
        }}
      />
      <div
        className="cpf-drop"
        role="button"
        tabIndex={0}
        onClick={() => !busy && fileRef.current?.click()}
        onKeyDown={(e) => {
          if ((e.key === "Enter" || e.key === " ") && !busy) {
            e.preventDefault();
            fileRef.current?.click();
          }
        }}
        style={{ outline: dragging ? "2px dashed var(--accent)" : undefined }}
        {...handlers}
      >
        <FileText size={22} />
        <span>
          {parsing
            ? "// LENDO O PDF…"
            : loaded.length > 0
              ? "// SOLTE OUTRO PDF PARA COMPLEMENTAR"
              : "// SOLTE O PDF AQUI OU CLIQUE PARA PREENCHER O CADASTRO"}
        </span>
        <small className="muted">
          RELATÓRIO DE CPF OU CONSULTA INTEGRADA · O ARQUIVO É LIDO E DESCARTADO
        </small>
      </div>
      {loaded.map((l, i) => (
        <div key={i} className="cpf-import-ok muted">
          ✓ {l.source} · {l.file}
          {l.generatedAt ? ` · ${formatBR(l.generatedAt)}` : ""}
          {l.records > 0 ? ` · ${l.records} REGISTRO(S) NO HISTÓRICO` : ""}
        </div>
      ))}
      {loaded.length > 0 && (
        <div className="cpf-import-ok muted">// CONFIRA OS CAMPOS NAS ABAS ANTES DE SALVAR</div>
      )}
      {err && (
        <div className="banner banner-error" style={{ marginTop: 6 }}>
          ⚠ {err}
        </div>
      )}
    </div>
  );
}

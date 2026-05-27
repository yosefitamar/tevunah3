"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { Building2, ImageUp, Loader2 } from "lucide-react";
import { useSystemSettings } from "@/contexts/SystemSettingsContext";
import { useModal } from "@/contexts/ModalContext";
import {
  brasaoPreviewURL,
  updateSystemSettings,
  uploadBrasao,
} from "@/lib/system-settings-api";
import type { ApiError } from "@/lib/api";

/**
 * Aba AGÊNCIA do painel admin. Permite configurar:
 *   - nome curto da agência (exibido no header/footer do shell)
 *   - título usado como ORIGEM padrão em RIs novos
 *   - brasão (consumido pelo gerador de PDF; não aparece na UI geral)
 *
 * As alterações no nome/título refletem em todo o sistema via SystemSettings-
 * Context (reload após save). O brasão grava em PHOTO_DIR/logo-sai.<ext>
 * pra o gerador de PDF reusar sem qualquer outra mudança.
 */
export default function AgencySettings() {
  const { settings, reload } = useSystemSettings();
  const modal = useModal();
  const [agency, setAgency] = useState("");
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);
  const [brasaoVer, setBrasaoVer] = useState(0);
  const [hasBrasao, setHasBrasao] = useState(false);
  const fileRef = useRef<HTMLInputElement | null>(null);

  // Hidrata os inputs quando o contexto carrega/recarrega.
  useEffect(() => {
    if (settings) {
      setAgency(settings.agency_name);
      setTitle(settings.document_title);
      setHasBrasao(!!settings.brasao_path);
    }
  }, [settings]);

  async function onSave(e: FormEvent) {
    e.preventDefault();
    if (!agency.trim()) {
      await modal.alert({
        variant: "error",
        title: "AGÊNCIA OBRIGATÓRIA",
        message: "O nome curto da agência não pode ficar vazio.",
      });
      return;
    }
    setBusy(true);
    try {
      await updateSystemSettings({
        agency_name: agency.trim(),
        document_title: title.trim(),
      });
      await reload();
    } catch (e) {
      await modal.alert({
        variant: "error",
        title: "FALHA AO SALVAR",
        message: (e as ApiError).message || "Erro desconhecido",
      });
    } finally {
      setBusy(false);
    }
  }

  async function onPickFile() {
    fileRef.current?.click();
  }

  async function onFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    e.target.value = ""; // permite re-selecionar o mesmo arquivo
    if (!f) return;
    if (!/^image\/(png|jpeg)$/.test(f.type)) {
      await modal.alert({
        variant: "error",
        title: "FORMATO NÃO SUPORTADO",
        message: "Envie um PNG ou JPEG.",
      });
      return;
    }
    setBusy(true);
    try {
      await uploadBrasao(f);
      await reload();
      setBrasaoVer((v) => v + 1); // bust cache do preview
      setHasBrasao(true);
    } catch (e) {
      await modal.alert({
        variant: "error",
        title: "FALHA NO UPLOAD",
        message: (e as ApiError).message || "Erro desconhecido",
      });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="agency-settings">
      <div className="section-title">
        DADOS DA AGÊNCIA
        <span style={{ color: "var(--fg-2)" }}>
          · IDENTIDADE INSTITUCIONAL DO SISTEMA
        </span>
      </div>

      <form className="agency-form" onSubmit={onSave}>
        <label className="form-field">
          <span>AGÊNCIA</span>
          <input
            type="text"
            value={agency}
            onChange={(e) => setAgency(e.target.value)}
            placeholder="ex.: SAI 2º BPRAIO"
            maxLength={120}
            required
          />
          <span className="form-hint">
            Exibido no header e no rodapé do sistema (◆ AGÊNCIA // TEVUNAH).
          </span>
        </label>

        <label className="form-field">
          <span>TÍTULO PARA DOCUMENTOS</span>
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="ex.: CCINT/ASINT/PMCE"
            maxLength={120}
          />
          <span className="form-hint">
            Preenche automaticamente o campo ORIGEM ao criar um relatório novo.
            Pode ser sobrescrito caso a caso.
          </span>
        </label>

        <div className="form-field">
          <span>BRASÃO</span>
          <div className="brasao-row">
            <div className="brasao-preview">
              {hasBrasao ? (
                <img
                  src={`${brasaoPreviewURL()}&v=${brasaoVer}`}
                  alt="Brasão atual"
                />
              ) : (
                <div className="brasao-empty">
                  <Building2 size={28} strokeWidth={1.4} />
                  <span>SEM BRASÃO</span>
                </div>
              )}
            </div>
            <div className="brasao-actions">
              <input
                ref={fileRef}
                type="file"
                accept="image/png,image/jpeg"
                onChange={onFileChange}
                style={{ display: "none" }}
              />
              <button
                type="button"
                className="btn btn-outline"
                onClick={onPickFile}
                disabled={busy}
              >
                <ImageUp size={14} strokeWidth={1.6} />
                {hasBrasao ? "SUBSTITUIR BRASÃO" : "ENVIAR BRASÃO"}
              </button>
              <span className="form-hint">
                PNG ou JPEG, até 2 MiB. Usado apenas no PDF gerado pelo sistema —
                não aparece na interface.
              </span>
            </div>
          </div>
        </div>

        <div className="agency-actions">
          <button type="submit" className="btn btn-primary" disabled={busy}>
            {busy ? (
              <>
                <Loader2 size={14} className="spin" strokeWidth={1.6} /> SALVANDO…
              </>
            ) : (
              "SALVAR ALTERAÇÕES"
            )}
          </button>
        </div>
      </form>
    </div>
  );
}

"use client";

import { useEffect, useRef, useState } from "react";
import {
  AlertOctagon,
  AlertTriangle,
  CheckCircle2,
  Info,
} from "lucide-react";
import type { ReactNode } from "react";
import TevunahLoader from "@/components/TevunahLoader";

// ─────────────────────────── Types ────────────────────────────

export type AlertInstance = {
  id: string;
  type: "alert";
  variant: "info" | "success" | "warning" | "error";
  title?: string;
  message: ReactNode;
  okLabel: string;
  autoCloseMs: number;
};

export type ConfirmInstance = {
  id: string;
  type: "confirm";
  variant: "info" | "warning" | "error";
  title?: string;
  message: ReactNode;
  confirmLabel: string;
  cancelLabel: string;
  danger: boolean;
};

export type PromptInstance = {
  id: string;
  type: "prompt";
  variant: "info" | "warning";
  title?: string;
  message?: ReactNode;
  label?: string;
  placeholder?: string;
  initialValue: string;
  inputType: "text" | "url" | "textarea";
  confirmLabel: string;
  cancelLabel: string;
  validate?: (value: string) => string | null;
};

export type LoadingInstance = {
  id: string;
  type: "loading";
  message: string;
};

export type ModalInstance =
  | AlertInstance
  | ConfirmInstance
  | PromptInstance
  | LoadingInstance;

type Props = {
  stack: ModalInstance[];
  onClose: (id: string, result?: unknown) => void;
};

// ─────────────────────────── Container ────────────────────────────

export default function ModalContainer({ stack, onClose }: Props) {
  // ESC fecha o topo (apenas alert/confirm/prompt — loading não cancela).
  useEffect(() => {
    if (stack.length === 0) return;
    const top = stack[stack.length - 1];
    if (top.type === "loading") return;

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (top.type === "confirm") onClose(top.id, false);
        else if (top.type === "prompt") onClose(top.id, null);
        else onClose(top.id);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [stack, onClose]);

  if (stack.length === 0) return null;

  return (
    <>
      {stack.map((m, i) => (
        <ModalView
          key={m.id}
          instance={m}
          onClose={onClose}
          // Apenas o topo recebe foco interativo; demais ficam empilhados visuais.
          topmost={i === stack.length - 1}
        />
      ))}
    </>
  );
}

// ─────────────────────────── ModalView ────────────────────────────

const VARIANT_HEADER: Record<string, string> = {
  info: "// AVISO",
  success: "// SUCESSO",
  warning: "// ATENÇÃO",
  error: "// ERRO",
  loading: "// OPERAÇÃO EM ANDAMENTO",
  prompt: "// ENTRADA",
};

function VariantIcon({ variant }: { variant: string }) {
  const props = { size: 40, strokeWidth: 1.5 };
  switch (variant) {
    case "success":
      return <CheckCircle2 {...props} />;
    case "warning":
      return <AlertTriangle {...props} />;
    case "error":
      return <AlertOctagon {...props} />;
    default:
      return <Info {...props} />;
  }
}

function ModalView({
  instance,
  onClose,
  topmost,
}: {
  instance: ModalInstance;
  onClose: (id: string, result?: unknown) => void;
  topmost: boolean;
}) {
  // Auto-close (apenas alert)
  useEffect(() => {
    if (instance.type !== "alert") return;
    if (!instance.autoCloseMs || instance.autoCloseMs <= 0) return;
    const t = window.setTimeout(() => onClose(instance.id), instance.autoCloseMs);
    return () => window.clearTimeout(t);
  }, [instance, onClose]);

  const variant = instance.type === "loading" ? "loading" : instance.variant;
  const isLoading = instance.type === "loading";
  const isConfirm = instance.type === "confirm";
  const isPrompt = instance.type === "prompt";
  const headerKey = isPrompt ? "prompt" : variant;

  function clickBackdrop() {
    if (!topmost) return;
    if (isLoading) return; // loading não pode ser cancelado pelo backdrop
    if (isConfirm) onClose(instance.id, false);
    else if (isPrompt) onClose(instance.id, null);
    else onClose(instance.id);
  }

  return (
    <div
      className="modal-backdrop"
      onClick={clickBackdrop}
      aria-hidden={!topmost}
    >
      <div
        className={
          "modal-card modal-card--" + variant +
          (topmost ? "" : " modal-card--stacked")
        }
        onClick={(e) => e.stopPropagation()}
        role={isConfirm ? "alertdialog" : "dialog"}
        aria-modal="true"
        aria-labelledby={`mhd-${instance.id}`}
      >
        <div className="modal-hd" id={`mhd-${instance.id}`}>
          <span>{VARIANT_HEADER[headerKey] ?? "// AVISO"}</span>
        </div>

        {isLoading ? (
          <div className="modal-bd modal-bd--loading">
            <TevunahLoader className="modal-loading-anim" />
            {instance.message && (
              <div className="modal-loading-message">{instance.message}</div>
            )}
          </div>
        ) : isPrompt ? (
          <PromptBody
            instance={instance as PromptInstance}
            onClose={onClose}
            topmost={topmost}
          />
        ) : (
          <div className="modal-bd">
            <div className={"modal-icon modal-icon--" + variant}>
              <VariantIcon variant={variant} />
            </div>
            <div className="modal-content">
              {instance.title && (
                <div className="modal-title">{instance.title}</div>
              )}
              <div className="modal-message">{instance.message}</div>
            </div>
          </div>
        )}

        {!isLoading && !isPrompt && (
          <div className="modal-ft">
            {isConfirm ? (
              <>
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => onClose(instance.id, false)}
                  autoFocus
                >
                  {instance.cancelLabel}
                </button>
                <button
                  type="button"
                  className={instance.danger ? "btn btn-danger" : "btn btn-primary"}
                  onClick={() => onClose(instance.id, true)}
                >
                  {instance.confirmLabel}
                </button>
              </>
            ) : (
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => onClose(instance.id)}
                autoFocus
              >
                {instance.type === "alert" ? instance.okLabel : "OK"}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ─────────────────────────── PromptBody ────────────────────────────

function PromptBody({
  instance,
  onClose,
  topmost,
}: {
  instance: PromptInstance;
  onClose: (id: string, result?: unknown) => void;
  topmost: boolean;
}) {
  const [value, setValue] = useState(instance.initialValue);
  const [err, setErr] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);

  useEffect(() => {
    if (topmost) inputRef.current?.focus();
  }, [topmost]);

  function submit() {
    const v = instance.inputType === "textarea" ? value : value.trim();
    if (instance.validate) {
      const e = instance.validate(v);
      if (e) {
        setErr(e);
        return;
      }
    }
    onClose(instance.id, v);
  }

  function onKey(e: React.KeyboardEvent<HTMLInputElement | HTMLTextAreaElement>) {
    // Enter envia em input/url; em textarea só com Cmd/Ctrl+Enter.
    if (e.key === "Enter") {
      if (instance.inputType === "textarea" && !(e.metaKey || e.ctrlKey)) return;
      e.preventDefault();
      submit();
    }
  }

  return (
    <>
      <div className="modal-bd">
        <div className={"modal-icon modal-icon--" + instance.variant}>
          <VariantIcon variant={instance.variant} />
        </div>
        <div className="modal-content">
          {instance.title && (
            <div className="modal-title">{instance.title}</div>
          )}
          {instance.message && (
            <div className="modal-message">{instance.message}</div>
          )}
          <label className="form-field" style={{ marginTop: 10 }}>
            {instance.label && <span>{instance.label}</span>}
            {instance.inputType === "textarea" ? (
              <textarea
                ref={(el) => {
                  inputRef.current = el;
                }}
                value={value}
                rows={4}
                placeholder={instance.placeholder}
                onChange={(e) => {
                  setValue(e.target.value);
                  if (err) setErr(null);
                }}
                onKeyDown={onKey}
              />
            ) : (
              <input
                ref={(el) => {
                  inputRef.current = el;
                }}
                type={instance.inputType}
                value={value}
                placeholder={instance.placeholder}
                onChange={(e) => {
                  setValue(e.target.value);
                  if (err) setErr(null);
                }}
                onKeyDown={onKey}
                autoComplete="off"
              />
            )}
          </label>
          {err && (
            <div className="banner banner-error" style={{ marginTop: 8 }}>
              ⚠ {err}
            </div>
          )}
        </div>
      </div>
      <div className="modal-ft">
        <button
          type="button"
          className="btn btn-ghost"
          onClick={() => onClose(instance.id, null)}
        >
          {instance.cancelLabel}
        </button>
        <button type="button" className="btn btn-primary" onClick={submit}>
          {instance.confirmLabel}
        </button>
      </div>
    </>
  );
}

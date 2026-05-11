"use client";

import { useEffect } from "react";
import {
  AlertOctagon,
  AlertTriangle,
  CheckCircle2,
  Info,
} from "lucide-react";
import type { ReactNode } from "react";
import TevunahLogo from "@/components/TevunahLogo";

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

export type LoadingInstance = {
  id: string;
  type: "loading";
  message: string;
};

export type ModalInstance = AlertInstance | ConfirmInstance | LoadingInstance;

type Props = {
  stack: ModalInstance[];
  onClose: (id: string, result?: unknown) => void;
};

// ─────────────────────────── Container ────────────────────────────

export default function ModalContainer({ stack, onClose }: Props) {
  // ESC fecha o topo (apenas alert/confirm — loading não pode ser cancelado).
  useEffect(() => {
    if (stack.length === 0) return;
    const top = stack[stack.length - 1];
    if (top.type === "loading") return;

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        if (top.type === "confirm") onClose(top.id, false);
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
};

function VariantIcon({ variant }: { variant: string }) {
  if (variant === "loading") {
    return <TevunahLogo className="modal-loading-logo" />;
  }
  const props = { size: 28, strokeWidth: 1.6 };
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

  function clickBackdrop() {
    if (!topmost) return;
    if (isLoading) return; // loading não pode ser cancelado pelo backdrop
    if (isConfirm) onClose(instance.id, false);
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
          <span>{VARIANT_HEADER[variant] ?? "// AVISO"}</span>
        </div>

        <div className="modal-bd">
          <div className={"modal-icon modal-icon--" + variant}>
            <VariantIcon variant={variant} />
          </div>
          <div className="modal-content">
            {instance.type !== "loading" && instance.title && (
              <div className="modal-title">{instance.title}</div>
            )}
            <div className="modal-message">{instance.message}</div>
          </div>
        </div>

        {!isLoading && (
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

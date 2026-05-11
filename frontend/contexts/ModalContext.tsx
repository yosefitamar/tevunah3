"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import ModalContainer, { type ModalInstance } from "@/components/modal/ModalContainer";

// ─────────────────────────── Types ────────────────────────────

export type AlertVariant = "info" | "success" | "warning" | "error";

export type AlertOptions = {
  title?: string;
  message: ReactNode;
  variant?: AlertVariant;
  /** Texto do botão; default "OK". */
  ok?: string;
  /** Auto-fecha após N ms. 0 = não auto-fecha (default 0, exceto success = 1500). */
  autoClose?: number;
};

export type ConfirmOptions = {
  title?: string;
  message: ReactNode;
  /** Variante visual; default "warning". */
  variant?: "warning" | "error" | "info";
  confirm?: string;
  cancel?: string;
  /** Marca o botão CONFIRMAR como destructive (vermelho). */
  danger?: boolean;
};

export type LoadingOptions = {
  message?: string;
  /** Atraso para mostrar o modal. Operações mais rápidas que isso passam invisíveis. Default 800ms. */
  thresholdMs?: number;
};

export type LoadingHandle = { close: () => void };

export type ModalAPI = {
  alert: (opts: AlertOptions) => Promise<void>;
  confirm: (opts: ConfirmOptions) => Promise<boolean>;
  loading: (opts?: LoadingOptions) => LoadingHandle;
};

// ─────────────────────────── Context ────────────────────────────

const ModalCtx = createContext<ModalAPI | null>(null);

export function useModal(): ModalAPI {
  const ctx = useContext(ModalCtx);
  if (!ctx) {
    throw new Error("useModal precisa estar dentro de <ModalProvider>");
  }
  return ctx;
}

// ─────────────────────────── Provider ────────────────────────────

let idSeq = 0;
const nextId = () => `m-${++idSeq}`;

export function ModalProvider({ children }: { children: ReactNode }) {
  const [stack, setStack] = useState<ModalInstance[]>([]);
  const resolversRef = useRef<Record<string, (value?: unknown) => void>>({});

  const close = useCallback((id: string, result?: unknown) => {
    setStack((s) => s.filter((m) => m.id !== id));
    const resolve = resolversRef.current[id];
    if (resolve) {
      delete resolversRef.current[id];
      resolve(result);
    }
  }, []);

  const alert = useCallback(
    (opts: AlertOptions) =>
      new Promise<void>((resolve) => {
        const id = nextId();
        const variant = opts.variant ?? "info";
        const autoCloseDefault = variant === "success" ? 1500 : 0;
        const autoClose = opts.autoClose ?? autoCloseDefault;

        resolversRef.current[id] = () => resolve();
        setStack((s) => [
          ...s,
          {
            id,
            type: "alert",
            variant,
            title: opts.title,
            message: opts.message,
            okLabel: opts.ok ?? "OK",
            autoCloseMs: autoClose,
          },
        ]);
      }),
    [],
  );

  const confirm = useCallback(
    (opts: ConfirmOptions) =>
      new Promise<boolean>((resolve) => {
        const id = nextId();
        resolversRef.current[id] = (v) => resolve(Boolean(v));
        setStack((s) => [
          ...s,
          {
            id,
            type: "confirm",
            variant: opts.variant ?? "warning",
            title: opts.title,
            message: opts.message,
            confirmLabel: opts.confirm ?? "CONFIRMAR",
            cancelLabel: opts.cancel ?? "CANCELAR",
            danger: Boolean(opts.danger),
          },
        ]);
      }),
    [],
  );

  const loading = useCallback((opts: LoadingOptions = {}): LoadingHandle => {
    const id = nextId();
    const threshold = opts.thresholdMs ?? 800;
    let shown = false;

    const timer = window.setTimeout(() => {
      shown = true;
      setStack((s) => [
        ...s,
        {
          id,
          type: "loading",
          message: opts.message ?? "PROCESSANDO…",
        },
      ]);
    }, threshold);

    return {
      close: () => {
        window.clearTimeout(timer);
        if (shown) {
          setStack((s) => s.filter((m) => m.id !== id));
        }
      },
    };
  }, []);

  const api = useMemo<ModalAPI>(
    () => ({ alert, confirm, loading }),
    [alert, confirm, loading],
  );

  return (
    <ModalCtx.Provider value={api}>
      {children}
      <ModalContainer stack={stack} onClose={close} />
    </ModalCtx.Provider>
  );
}

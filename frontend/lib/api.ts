// Cliente HTTP para a API Tevunah. Sempre passa por /api/* (rewrite do Next),
// então é same-origin e o cookie HttpOnly de sessão é enviado automaticamente.

export type ApiError = Error & { status: number };

function makeError(message: string, status: number): ApiError {
  const e = new Error(message) as ApiError;
  e.status = status;
  return e;
}

type Envelope<T> = { success: boolean; data?: T; message?: string; errors?: unknown };

// Handler global para 401: chamado sempre que qualquer endpoint devolve 401.
// O AuthContext registra um handler que distingue "não logado" (estado inicial)
// de "sessão expirou no meio do uso" — neste último, ele dispara o overlay
// de re-autenticação sem mexer no resto do shell.
type UnauthorizedHandler = (path: string, message: string) => void;
let unauthorizedHandler: UnauthorizedHandler | null = null;
export function setUnauthorizedHandler(fn: UnauthorizedHandler | null) {
  unauthorizedHandler = fn;
}

// Handler global de expiração: o backend envia X-Session-Expires-At em cada
// resposta autenticada (após Touch). O AuthContext armazena o valor pra
// alimentar o timer da topbar — sem polling, refresh natural por atividade.
type SessionExpiresHandler = (expiresAt: Date) => void;
let sessionExpiresHandler: SessionExpiresHandler | null = null;
export function setSessionExpiresHandler(fn: SessionExpiresHandler | null) {
  sessionExpiresHandler = fn;
}

export async function api<T = unknown>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(init.headers ?? {}),
    },
  });

  let body: Envelope<T> = { success: false };
  try {
    body = (await res.json()) as Envelope<T>;
  } catch {
    // resposta sem corpo (ex.: 204) — segue
  }

  if (res.status === 401) {
    // Ignora endpoints de auth (login/me/logout) — eles têm seu próprio fluxo
    // e não devem disparar o overlay (ex.: 401 em /login = credencial inválida).
    if (!path.startsWith("/api/auth/")) {
      unauthorizedHandler?.(path, body.message ?? "sessão expirada");
    }
  } else if (res.ok) {
    // Header de expiração: emitido pelo middleware autenticado após Touch.
    const exp = res.headers.get("X-Session-Expires-At");
    if (exp) {
      const d = new Date(exp);
      if (!Number.isNaN(d.getTime())) sessionExpiresHandler?.(d);
    }
  }

  if (!res.ok) {
    throw makeError(body.message ?? `HTTP ${res.status}`, res.status);
  }
  return (body.data ?? (undefined as unknown as T)) as T;
}

// Cliente HTTP para a API Tevunah. Sempre passa por /api/* (rewrite do Next),
// então é same-origin e o cookie HttpOnly de sessão é enviado automaticamente.

export type ApiError = Error & { status: number };

function makeError(message: string, status: number): ApiError {
  const e = new Error(message) as ApiError;
  e.status = status;
  return e;
}

type Envelope<T> = { success: boolean; data?: T; message?: string; errors?: unknown };

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

  if (!res.ok) {
    throw makeError(body.message ?? `HTTP ${res.status}`, res.status);
  }
  return (body.data ?? (undefined as unknown as T)) as T;
}

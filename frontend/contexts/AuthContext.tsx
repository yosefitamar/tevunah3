"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  api,
  setSessionExpiresHandler,
  setUnauthorizedHandler,
  type ApiError,
} from "@/lib/api";
import type { RoleRow, User } from "@/lib/types";
import { roleLabel as resolveRoleLabel } from "@/lib/types";
import { listRoles } from "@/lib/roles-api";

type LoginInput = { email: string; password: string; totp_code?: string };

// Dados retornados pelo backend quando o agente precisa configurar TOTP
// no primeiro login (após reset por admin). Frontend usa pra montar o QR.
export type PendingTOTPSetup = {
  secret: string;
  email: string;
};

type AuthState = {
  user: User | null;
  /** Papéis cadastrados (dinâmicos, de /api/roles). Vazio até carregar. */
  roles: RoleRow[];
  /** Resolve o rótulo (UPPERCASE) de um papel pela lista dinâmica. */
  roleLabel: (code: string) => string;
  /** Recarrega a lista de papéis (após CRUD na tela de gestão). */
  refreshRoles: () => Promise<void>;
  loading: boolean;
  error: string | null;
  /** True quando a sessão expirou mid-uso e o overlay de re-auth está ativo. */
  sessionExpired: boolean;
  /** Quando a sessão atual expira por inatividade (atualizado a cada call). */
  sessionExpiresAt: Date | null;
  /** Setup TOTP pendente (devolvido pelo login) — usado pela tela de enrollment. */
  pendingTOTPSetup: PendingTOTPSetup | null;
  clearPendingTOTPSetup: () => void;
  login: (input: LoginInput) => Promise<void>;
  logout: () => Promise<void>;
  refreshUser: () => Promise<void>;
};

const AuthCtx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [roles, setRoles] = useState<RoleRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [sessionExpired, setSessionExpired] = useState(false);
  const [sessionExpiresAt, setSessionExpiresAt] = useState<Date | null>(null);
  const [pendingTOTPSetup, setPendingTOTPSetup] = useState<PendingTOTPSetup | null>(null);

  // Ref pra ler o estado atual dentro do handler global (evita rebind).
  const userRef = useRef<User | null>(null);
  useEffect(() => {
    userRef.current = user;
  }, [user]);

  const refresh = useCallback(async () => {
    try {
      const data = await api<{ user: User; totp_setup?: PendingTOTPSetup }>(
        "/api/auth/me",
      );
      setUser(data.user);
      setPendingTOTPSetup(data.totp_setup ?? null);
    } catch (e) {
      const err = e as ApiError;
      if (err.status !== 401) {
        // 401 é o estado normal "não autenticado" — não pinta como erro.
        setError(err.message);
      }
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Registra handler global de 401. Só dispara o overlay se já havia um
  // usuário logado (i.e., sessão expirou mid-uso, não estado inicial).
  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (userRef.current) {
        setSessionExpired(true);
        setSessionExpiresAt(null);
      }
    });
    return () => setUnauthorizedHandler(null);
  }, []);

  // Recebe a expiração de cada resposta autenticada (Touch refresh).
  useEffect(() => {
    setSessionExpiresHandler((d) => setSessionExpiresAt(d));
    return () => setSessionExpiresHandler(null);
  }, []);

  // Quando o relógio bate na expiração, dispara o overlay sem esperar um 401.
  // Sem isso, um usuário inativo veria o timer chegar a 00:00 e nada
  // aconteceria até a próxima chamada manual. Aqui agendamos o gatilho.
  useEffect(() => {
    if (!sessionExpiresAt || !userRef.current) return;
    const ms = sessionExpiresAt.getTime() - Date.now();
    if (ms <= 0) {
      setSessionExpired(true);
      return;
    }
    const id = window.setTimeout(() => setSessionExpired(true), ms);
    return () => window.clearTimeout(id);
  }, [sessionExpiresAt]);

  // Heartbeat defensivo: o setTimeout acima pode ser pausado por throttling
  // do browser (aba inativa, sleep do laptop) e ressuscitar atrasado. Esse
  // tick reavalia o relógio a cada 2s e, ao detectar expiração já estourada,
  // dispara o overlay imediatamente. Sem rede, custo desprezível.
  useEffect(() => {
    if (!sessionExpiresAt || !user || sessionExpired) return;
    const tick = () => {
      if (sessionExpiresAt.getTime() <= Date.now()) setSessionExpired(true);
    };
    const id = window.setInterval(tick, 2000);
    return () => window.clearInterval(id);
  }, [sessionExpiresAt, user, sessionExpired]);

  // Sleep do laptop / aba em background congelam timers; quando o usuário
  // volta ao foco, reavaliamos o relógio na hora pra não esperar o próximo
  // tick do heartbeat.
  useEffect(() => {
    if (!user) return;
    const onVis = () => {
      if (document.visibilityState !== "visible") return;
      if (sessionExpiresAt && sessionExpiresAt.getTime() <= Date.now()) {
        setSessionExpired(true);
      }
    };
    document.addEventListener("visibilitychange", onVis);
    window.addEventListener("focus", onVis);
    return () => {
      document.removeEventListener("visibilitychange", onVis);
      window.removeEventListener("focus", onVis);
    };
  }, [user, sessionExpiresAt]);

  const login = useCallback(async (input: LoginInput) => {
    setError(null);
    const data = await api<{
      user: User;
      token: string;
      expires_in: number;
      totp_setup?: PendingTOTPSetup;
    }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setUser(data.user);
    setSessionExpired(false);
    setSessionExpiresAt(new Date(Date.now() + data.expires_in * 1000));
    setPendingTOTPSetup(data.totp_setup ?? null);
  }, []);

  const refreshUser = useCallback(async () => {
    const data = await api<{ user: User; totp_setup?: PendingTOTPSetup }>(
      "/api/auth/me",
    );
    setUser(data.user);
    setPendingTOTPSetup(data.totp_setup ?? null);
  }, []);

  const refreshRoles = useCallback(async () => {
    try {
      const data = await listRoles();
      setRoles(data.items ?? []);
    } catch {
      // /api/roles exige sessão; em 401 (deslogado) só ignora.
    }
  }, []);

  // Carrega os papéis quando há usuário; limpa ao deslogar.
  useEffect(() => {
    if (user) refreshRoles();
    else setRoles([]);
  }, [user, refreshRoles]);

  const roleLabel = useCallback(
    (code: string) => resolveRoleLabel(code, roles),
    [roles],
  );

  const clearPendingTOTPSetup = useCallback(() => setPendingTOTPSetup(null), []);

  const logout = useCallback(async () => {
    try {
      await api<void>("/api/auth/logout", { method: "POST" });
    } catch {
      // se a sessão já expirou no servidor, segue
    }
    setUser(null);
    setSessionExpired(false);
    setSessionExpiresAt(null);
    setPendingTOTPSetup(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({
      user,
      roles,
      roleLabel,
      refreshRoles,
      loading,
      error,
      sessionExpired,
      sessionExpiresAt,
      pendingTOTPSetup,
      clearPendingTOTPSetup,
      login,
      logout,
      refreshUser,
    }),
    [
      user,
      roles,
      roleLabel,
      refreshRoles,
      loading,
      error,
      sessionExpired,
      sessionExpiresAt,
      pendingTOTPSetup,
      clearPendingTOTPSetup,
      login,
      logout,
      refreshUser,
    ],
  );

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth deve ser usado dentro de <AuthProvider>");
  return ctx;
}

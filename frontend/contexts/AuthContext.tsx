"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api, type ApiError } from "@/lib/api";
import type { User } from "@/lib/types";

type LoginInput = { email: string; password: string; totp_code: string };

type AuthState = {
  user: User | null;
  loading: boolean;
  error: string | null;
  login: (input: LoginInput) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthCtx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const data = await api<{ user: User }>("/api/auth/me");
      setUser(data.user);
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

  const login = useCallback(async (input: LoginInput) => {
    setError(null);
    const data = await api<{ user: User; token: string; expires_in: number }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setUser(data.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await api<void>("/api/auth/logout", { method: "POST" });
    } catch {
      // se a sessão já expirou no servidor, segue
    }
    setUser(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({ user, loading, error, login, logout }),
    [user, loading, error, login, logout],
  );

  return <AuthCtx.Provider value={value}>{children}</AuthCtx.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthCtx);
  if (!ctx) throw new Error("useAuth deve ser usado dentro de <AuthProvider>");
  return ctx;
}

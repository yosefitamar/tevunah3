"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  getSystemSettings,
  type SystemSettings,
} from "@/lib/system-settings-api";

// Contexto leve com as configurações do sistema (nome da agência + título
// padrão de documentos). Carregado uma vez após autenticação e cacheado em
// memória. Componentes consomem via useSystemSettings().
//
// Fallbacks: enquanto não carregou (ou em erro), expõe strings vazias —
// caller decide o que mostrar (geralmente "—" ou skeleton). O fallback NÃO
// é mais "SAI 2º BPRAIO" pra não enganar quem migrou.

type Ctx = {
  settings: SystemSettings | null;
  loading: boolean;
  reload: () => Promise<void>;
  /** Atualiza o cache local sem refetch (uso após PUT). */
  setSettings: (s: SystemSettings) => void;
};

const SystemSettingsContext = createContext<Ctx>({
  settings: null,
  loading: true,
  reload: async () => {},
  setSettings: () => {},
});

export function SystemSettingsProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<SystemSettings | null>(null);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const { settings } = await getSystemSettings();
      setSettings(settings);
    } catch {
      setSettings(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return (
    <SystemSettingsContext.Provider value={{ settings, loading, reload, setSettings }}>
      {children}
    </SystemSettingsContext.Provider>
  );
}

export function useSystemSettings(): Ctx {
  return useContext(SystemSettingsContext);
}

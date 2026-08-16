"use client";

import { createContext, useCallback, useContext, useMemo, useState } from "react";
import type { ModuleId } from "@/lib/nav";

/**
 * Alvo de navegação entre módulos. O app é uma SPA de estado local (sem
 * rotas), então saltar da ficha de uma entidade para o mapa exige um canal
 * explícito: quem navega diz para onde vai e o que deve ficar em foco.
 */
export type NavTarget = {
  module: ModuleId;
  /** Ocorrência a focar no mapa (centraliza o pino e abre o dossiê). */
  incidentId?: string;
};

type NavigationValue = {
  active: ModuleId;
  /** Alvo pendente para o módulo que acabou de ser aberto (consumo único). */
  pending: NavTarget | null;
  navigate: (target: NavTarget) => void;
  setActive: (m: ModuleId) => void;
  /** O módulo de destino chama isto após aplicar o foco, evitando repetição. */
  consumePending: () => void;
};

const Ctx = createContext<NavigationValue | null>(null);

export function NavigationProvider({
  active,
  setActive,
  children,
}: {
  active: ModuleId;
  setActive: (m: ModuleId) => void;
  children: React.ReactNode;
}) {
  const [pending, setPending] = useState<NavTarget | null>(null);

  const navigate = useCallback(
    (target: NavTarget) => {
      setPending(target);
      setActive(target.module);
    },
    [setActive],
  );

  const consumePending = useCallback(() => setPending(null), []);

  const value = useMemo(
    () => ({ active, pending, navigate, setActive, consumePending }),
    [active, pending, navigate, setActive, consumePending],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useNavigation(): NavigationValue {
  const v = useContext(Ctx);
  if (!v) throw new Error("useNavigation fora de NavigationProvider");
  return v;
}

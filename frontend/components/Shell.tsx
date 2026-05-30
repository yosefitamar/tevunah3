"use client";

import { useEffect, useState } from "react";
import { MODULE_TITLES, type ModuleId } from "@/lib/nav";
import { type PaletteId } from "@/lib/palettes";
import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { SystemSettingsProvider, useSystemSettings } from "@/contexts/SystemSettingsContext";
import { ModalProvider } from "@/contexts/ModalContext";
import Sidebar from "./Sidebar";
import Topbar from "./Topbar";
import PaletteSwitcher from "./PaletteSwitcher";
import LoginScreen from "./LoginScreen";
import TOTPSetupScreen from "./TOTPSetupScreen";
import ChangePasswordScreen from "./ChangePasswordScreen";
import SessionExpiredOverlay from "./SessionExpiredOverlay";
import Dashboard from "./dashboard/Dashboard";
import {
  ScreenAdmin,
  ScreenAgentes,
  ScreenAprovacoes,
  ScreenAuditoria,
  ScreenEntidades,
  ScreenOcorrencias,
  ScreenRelatorios,
} from "./screens";
import SandboxModais from "./sandbox/SandboxModais";

const VIEWS: Record<ModuleId, React.ComponentType> = {
  dashboard: Dashboard,
  entidades: ScreenEntidades,
  ocorrencias: ScreenOcorrencias,
  relatorios: ScreenRelatorios,
  agentes: ScreenAgentes,
  aprovacoes: ScreenAprovacoes,
  auditoria: ScreenAuditoria,
  admin: ScreenAdmin,
  sandbox: SandboxModais,
};

function AuthenticatedShell() {
  const { sessionExpired } = useAuth();
  const { settings } = useSystemSettings();
  const [active, setActive] = useState<ModuleId>("dashboard");
  const [collapsed, setCollapsed] = useState(false);
  const [palette, setPalette] = useState<PaletteId>("phosphor");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const agencyLabel = settings?.agency_name || "—";

  useEffect(() => {
    document.documentElement.setAttribute("data-palette", palette);
  }, [palette]);

  const View = VIEWS[active];
  const sideW = collapsed ? "62px" : "248px";

  return (
    <div className="shell" style={{ ["--side-w" as string]: sideW } as React.CSSProperties}>
      <div className="classification">
        <span>◆ {agencyLabel}</span>
        <span className="sep">//</span>
        <span>TEVUNAH</span>
      </div>

      <div className="app-row">
        <Sidebar
          active={active}
          setActive={setActive}
          collapsed={collapsed}
          setCollapsed={setCollapsed}
        />
        <div style={{ display: "flex", flexDirection: "column", minWidth: 0, minHeight: 0 }}>
          <Topbar active={active} onToggleSettings={() => setSettingsOpen((v) => !v)} />
          <main className="content" data-screen-label={MODULE_TITLES[active]}>
            <View />
          </main>
        </div>
      </div>

      <div className="classification bottom">
        <span>◆ {agencyLabel} // TEVUNAH</span>
        <span className="sep">//</span>
        <span>SESSÃO 0x4F-A91C · TERM-04 · 192.168.42.18</span>
        <span className="sep">//</span>
        <span>USO MONITORADO</span>
      </div>

      {settingsOpen && <PaletteSwitcher palette={palette} setPalette={setPalette} />}
      {sessionExpired && <SessionExpiredOverlay />}
    </div>
  );
}

function AuthGate() {
  const { user, loading, pendingTOTPSetup } = useAuth();
  if (loading) return <div className="gate-loading">// AUTENTICANDO SESSÃO…</div>;
  if (!user) return <LoginScreen />;
  // Setup pendente de TOTP toma precedência — sem secret confirmado o agente
  // não tem 2FA ativo; só libera o shell após confirmar o enrollment.
  if (user.must_setup_totp && pendingTOTPSetup) return <TOTPSetupScreen />;
  if (user.must_change_password) return <ChangePasswordScreen />;
  return <AuthenticatedShell />;
}

export default function Shell() {
  return (
    <AuthProvider>
      <SystemSettingsProvider>
        <ModalProvider>
          <AuthGate />
        </ModalProvider>
      </SystemSettingsProvider>
    </AuthProvider>
  );
}

"use client";

import { useEffect, useState } from "react";
import { MODULE_TITLES, type ModuleId } from "@/lib/nav";
import { type PaletteId } from "@/lib/palettes";
import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import Sidebar from "./Sidebar";
import Topbar from "./Topbar";
import PaletteSwitcher from "./PaletteSwitcher";
import LoginScreen from "./LoginScreen";
import Dashboard from "./dashboard/Dashboard";
import { ScreenAgentes, ScreenAuditoria, ScreenAdmin } from "./screens";

const VIEWS: Record<ModuleId, React.ComponentType> = {
  dashboard: Dashboard,
  agentes: ScreenAgentes,
  auditoria: ScreenAuditoria,
  admin: ScreenAdmin,
};

function AuthenticatedShell() {
  const [active, setActive] = useState<ModuleId>("dashboard");
  const [collapsed, setCollapsed] = useState(false);
  const [palette, setPalette] = useState<PaletteId>("phosphor");
  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => {
    document.documentElement.setAttribute("data-palette", palette);
  }, [palette]);

  const View = VIEWS[active];
  const sideW = collapsed ? "62px" : "248px";

  return (
    <div className="shell" style={{ ["--side-w" as string]: sideW } as React.CSSProperties}>
      <div className="classification">
        <span>◆ SAI 2º BPRAIO</span>
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
        <span>◆ SAI 2º BPRAIO // TEVUNAH</span>
        <span className="sep">//</span>
        <span>SESSÃO 0x4F-A91C · TERM-04 · 192.168.42.18</span>
        <span className="sep">//</span>
        <span>USO MONITORADO</span>
      </div>

      {settingsOpen && <PaletteSwitcher palette={palette} setPalette={setPalette} />}
    </div>
  );
}

function AuthGate() {
  const { user, loading } = useAuth();
  if (loading) return <div className="gate-loading">// AUTENTICANDO SESSÃO…</div>;
  if (!user) return <LoginScreen />;
  return <AuthenticatedShell />;
}

export default function Shell() {
  return (
    <AuthProvider>
      <AuthGate />
    </AuthProvider>
  );
}

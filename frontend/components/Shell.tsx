"use client";

import { useEffect, useState } from "react";
import { IS_DEV, MODULE_TITLES, type ModuleId } from "@/lib/nav";
import { type PaletteId } from "@/lib/palettes";
import { DEFAULT_UI_SCALE, applyUiScale, readUiScale, type UiScale } from "@/lib/ui-scale";
import { deviceLabel, readDeviceId } from "@/lib/device-id";
import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import { SystemSettingsProvider, useSystemSettings } from "@/contexts/SystemSettingsContext";
import { ModalProvider } from "@/contexts/ModalContext";
import { NavigationProvider } from "@/contexts/NavigationContext";
import Sidebar from "./Sidebar";
import Topbar from "./Topbar";
import AppearanceMenu from "./AppearanceMenu";
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
  ScreenInformes,
  ScreenMapa,
  ScreenOcorrencias,
  ScreenRelatorios,
} from "./screens";
import SandboxModais from "./sandbox/SandboxModais";

const VIEWS: Record<ModuleId, React.ComponentType> = {
  dashboard: Dashboard,
  entidades: ScreenEntidades,
  ocorrencias: ScreenOcorrencias,
  mapa: ScreenMapa,
  relatorios: ScreenRelatorios,
  informes: ScreenInformes,
  agentes: ScreenAgentes,
  aprovacoes: ScreenAprovacoes,
  auditoria: ScreenAuditoria,
  admin: ScreenAdmin,
  sandbox: SandboxModais,
};

function AuthenticatedShell() {
  const { sessionExpired, sessionInfo } = useAuth();
  const { settings } = useSystemSettings();
  const [active, setActive] = useState<ModuleId>("dashboard");
  const [collapsed, setCollapsed] = useState(false);
  const [palette, setPalette] = useState<PaletteId>("phosphor");
  // O script do <head> já aplicou a escala salva antes da primeira pintura;
  // aqui o estado só se alinha com o que está no documento, para o popover
  // marcar a opção certa. Ler no useState inicial quebraria a hidratação (o
  // servidor não tem localStorage).
  const [scale, setScale] = useState<UiScale>(DEFAULT_UI_SCALE);
  const [settingsOpen, setSettingsOpen] = useState(false);
  // O ID do terminal vive no localStorage: ler no primeiro render quebraria a
  // hidratação (o servidor não tem storage), então entra depois do mount.
  const [deviceId, setDeviceId] = useState("");
  const agencyLabel = settings?.agency_name || "—";

  useEffect(() => {
    setDeviceId(readDeviceId());
  }, []);

  // Rodapé de rastreabilidade: só entra o que já é conhecido. Enquanto o /me
  // não respondeu (ou o storage está bloqueado), o campo some em vez de
  // exibir placeholder — barra de classificação não mente.
  const traceLabel = [
    sessionInfo?.id ? `SESSÃO ${sessionInfo.id}` : null,
    deviceLabel(deviceId) || null,
    sessionInfo?.ip || null,
  ]
    .filter(Boolean)
    .join(" · ");

  useEffect(() => {
    document.documentElement.setAttribute("data-palette", palette);
  }, [palette]);

  useEffect(() => {
    const saved = readUiScale();
    setScale(saved);
    // Reaplica: cobre o caso do script do <head> ter falhado (CSP, storage
    // bloqueado) e garante que --ui-scale esteja publicada para o CSS.
    applyUiScale(saved);
  }, []);

  // Sandbox é só de desenvolvimento — em prod, qualquer tentativa de ativá-lo
  // cai no Dashboard (o item nem aparece no menu, isto é defesa em profundidade).
  const safeActive: ModuleId = active === "sandbox" && !IS_DEV ? "dashboard" : active;
  const View = VIEWS[safeActive];
  const sideW = collapsed ? "62px" : "248px";

  return (
    <NavigationProvider active={safeActive} setActive={setActive}>
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
          <Topbar active={safeActive} onToggleSettings={() => setSettingsOpen((v) => !v)} />
          <main className="content" data-screen-label={MODULE_TITLES[safeActive]}>
            <View />
          </main>
        </div>
      </div>

      {/* A barra de cima já identifica agência e sistema; aqui só entra o que
          muda de sessão para sessão. */}
      <div className="classification bottom">
        {traceLabel && (
          <>
            <span title={sessionStartedTitle(sessionInfo?.started_at)}>{traceLabel}</span>
            <span className="sep">//</span>
          </>
        )}
        <span>USO MONITORADO</span>
      </div>

      {settingsOpen && (
        <AppearanceMenu
          palette={palette}
          setPalette={setPalette}
          scale={scale}
          setScale={setScale}
        />
      )}
      {sessionExpired && <SessionExpiredOverlay />}
    </div>
    </NavigationProvider>
  );
}

// Tooltip do rodapé: hora local de abertura da sessão. Data ausente ou
// inválida devolve undefined, e o title simplesmente não aparece.
function sessionStartedTitle(startedAt?: string): string | undefined {
  if (!startedAt) return undefined;
  const d = new Date(startedAt);
  if (Number.isNaN(d.getTime())) return undefined;
  return `Sessão aberta em ${d.toLocaleString("pt-BR")}`;
}

function AuthGate() {
  const { user, loading, pendingTOTPSetup } = useAuth();
  if (loading) return <div className="gate-loading">// AUTENTICANDO SESSÃO…</div>;
  if (!user) return <LoginScreen />;
  // Com as duas pendências ativas, a troca de senha vem primeiro: a senha
  // temporária deixa de circular o quanto antes e o agente só vê o QR do
  // novo TOTP depois de assumir uma credencial própria.
  if (user.must_change_password) return <ChangePasswordScreen />;
  if (user.must_setup_totp && pendingTOTPSetup) return <TOTPSetupScreen />;
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

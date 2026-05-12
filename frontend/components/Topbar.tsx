"use client";

import { useEffect, useState } from "react";
import { Bell, LogOut, Maximize2, Minimize2, Settings2 } from "lucide-react";
import { MODULE_TITLES, type ModuleId } from "@/lib/nav";
import { useAuth } from "@/contexts/AuthContext";
import { clearanceLabel, primaryRole } from "@/lib/types";
import SessionTimer from "./SessionTimer";

// Relógio em horário de Fortaleza (BRT, UTC-3, sem DST).
const FORTALEZA_FMT = new Intl.DateTimeFormat("pt-BR", {
  timeZone: "America/Fortaleza",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});

function useClock(): string {
  const [t, setT] = useState<string>("--:--:--");
  useEffect(() => {
    const tick = () => setT(FORTALEZA_FMT.format(new Date()));
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, []);
  return t;
}

function useFullscreen() {
  const [isFull, setIsFull] = useState(false);
  useEffect(() => {
    const handler = () => setIsFull(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", handler);
    return () => document.removeEventListener("fullscreenchange", handler);
  }, []);
  const toggle = async () => {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
    } else {
      await document.documentElement.requestFullscreen();
    }
  };
  return { isFull, toggle };
}

type Props = {
  active: ModuleId;
  onToggleSettings: () => void;
  notifications?: number;
};

export default function Topbar({ active, onToggleSettings, notifications = 0 }: Props) {
  const clock = useClock();
  const { user, logout } = useAuth();
  const { isFull, toggle: toggleFull } = useFullscreen();

  return (
    <header className="topbar">
      <div className="crumb">
        <span className="module-name">{MODULE_TITLES[active]}</span>
      </div>
      <div className="search">
        <span style={{ color: "var(--accent)" }}>⌕</span>
        <input placeholder="buscar entidades, operações, relatórios…" />
        <span className="kbd">⌘ K</span>
      </div>
      <div className="stat">
        <span className="lbl">FORTALEZA</span>
        <span className="val" style={{ fontFeatureSettings: '"tnum"' }}>
          {clock}
        </span>
      </div>
      <SessionTimer />
      <div className="stat">
        <span className="lbl">UPLINK</span>
        <span className="val">
          <span className="dot"></span>NOMINAL
        </span>
      </div>

      <div className="actions">
        <button type="button" className="action-btn" title="Notificações" aria-label="Notificações">
          <Bell size={16} strokeWidth={1.6} />
          {notifications > 0 && (
            <span className="action-badge">{notifications > 99 ? "99+" : notifications}</span>
          )}
        </button>
        <button
          type="button"
          className="action-btn"
          title={isFull ? "Sair de tela cheia" : "Tela cheia"}
          aria-label={isFull ? "Sair de tela cheia" : "Tela cheia"}
          onClick={toggleFull}
        >
          {isFull ? <Minimize2 size={16} strokeWidth={1.6} /> : <Maximize2 size={16} strokeWidth={1.6} />}
        </button>
        <button
          type="button"
          className="action-btn"
          title="Configurações rápidas"
          aria-label="Configurações rápidas"
          onClick={onToggleSettings}
        >
          <Settings2 size={16} strokeWidth={1.6} />
        </button>
      </div>

      {user && (
        <div className="user">
          <span className="name">{user.display_name.toUpperCase()}</span>
          <span className="role">
            {clearanceLabel(user.clearance_level)} · {primaryRole(user)}
          </span>
        </div>
      )}

      <button
        type="button"
        className="action-btn logout-btn"
        title="Encerrar sessão"
        aria-label="Encerrar sessão"
        onClick={logout}
      >
        <LogOut size={16} strokeWidth={1.6} />
      </button>
    </header>
  );
}

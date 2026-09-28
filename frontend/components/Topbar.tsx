"use client";

import { useEffect, useState } from "react";
import { LogOut, Maximize2, Minimize2, Settings2 } from "lucide-react";
import { MODULE_TITLES, type ModuleId } from "@/lib/nav";
import { useAuth } from "@/contexts/AuthContext";
import { clearanceLabel, primaryRole } from "@/lib/types";
import SessionTimer from "./SessionTimer";

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
};

/**
 * Barra superior: só o que o agente usa ou precisa monitorar. Relógio,
 * indicador de uplink, busca e sino saíram — eram ilustrativos (a busca não
 * tinha handler, o sino nunca recebia contagem) e disputavam largura com o
 * título do módulo. O timer de sessão fica: é ele que avisa do logout.
 */
export default function Topbar({ active, onToggleSettings }: Props) {
  const { user, logout } = useAuth();
  const { isFull, toggle: toggleFull } = useFullscreen();

  return (
    <header className="topbar">
      <div className="crumb">
        <span className="module-name">{MODULE_TITLES[active]}</span>
      </div>
      <SessionTimer />

      <div className="actions">
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

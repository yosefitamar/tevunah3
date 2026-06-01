"use client";

import { useState } from "react";
import { Building2, ShieldAlert, SlidersHorizontal, Monitor, Users } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { canAccessAdmin } from "@/lib/permissions";
import AgencySettings from "./AgencySettings";
import PermissionsMatrix from "./PermissionsMatrix";
import RolesManagement from "./RolesManagement";
import DevicesPlaceholder from "./DevicesPlaceholder";

type Tab = "agencia" | "papeis" | "matriz" | "dispositivos";

export default function AdminScreen() {
  const { user: me } = useAuth();
  const [tab, setTab] = useState<Tab>("agencia");

  if (!canAccessAdmin(me)) {
    return (
      <div className="placeholder">
        <div className="ph-tag">// MOD-99 / ADMIN</div>
        <div className="ph-ttl" style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <ShieldAlert size={22} /> ACESSO RESTRITO
        </div>
        <div className="ph-sub">
          Esta seção é restrita ao administrador. Configurações do sistema, matriz
          RBAC e gerenciamento de dispositivos não são acessíveis a outros papéis.
        </div>
      </div>
    );
  }

  return (
    <div className="screen-fill">
      <div className="section-title">
        ADMINISTRAÇÃO DO SISTEMA
        <span style={{ color: "var(--fg-2)" }}>· CONFIG · RBAC · DISPOSITIVOS</span>
      </div>

      <div className="admin-tabs">
        <button
          type="button"
          className={"admin-tab" + (tab === "agencia" ? " admin-tab--on" : "")}
          onClick={() => setTab("agencia")}
        >
          <Building2 size={13} strokeWidth={1.6} />
          <span>AGÊNCIA</span>
        </button>
        <button
          type="button"
          className={"admin-tab" + (tab === "papeis" ? " admin-tab--on" : "")}
          onClick={() => setTab("papeis")}
        >
          <Users size={13} strokeWidth={1.6} />
          <span>PAPÉIS</span>
        </button>
        <button
          type="button"
          className={"admin-tab" + (tab === "matriz" ? " admin-tab--on" : "")}
          onClick={() => setTab("matriz")}
        >
          <SlidersHorizontal size={13} strokeWidth={1.6} />
          <span>MATRIZ RBAC</span>
        </button>
        <button
          type="button"
          className={"admin-tab" + (tab === "dispositivos" ? " admin-tab--on" : "")}
          onClick={() => setTab("dispositivos")}
        >
          <Monitor size={13} strokeWidth={1.6} />
          <span>DISPOSITIVOS</span>
          <span className="admin-tab-hint">EM BREVE</span>
        </button>
      </div>

      {tab === "agencia" && <AgencySettings />}
      {tab === "papeis" && <RolesManagement />}
      {tab === "matriz" && <PermissionsMatrix />}
      {tab === "dispositivos" && <DevicesPlaceholder />}
    </div>
  );
}

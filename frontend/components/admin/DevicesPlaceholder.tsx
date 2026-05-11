"use client";

import { Monitor } from "lucide-react";

export default function DevicesPlaceholder() {
  return (
    <div className="placeholder placeholder--inset">
      <div className="ph-tag">// SUB-MOD / DISPOSITIVOS</div>
      <div className="ph-ttl" style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <Monitor size={22} /> DEVICE BINDING
      </div>
      <div className="ph-sub">
        Cada estação será cadastrada na primeira tentativa de login e dependerá de
        aprovação para se tornar uma máquina de confiança. Aprovação por gestor ou
        administrador, com fingerprint passivo registrado no audit log. Tela em
        construção — schema e fluxo já desenhados.
      </div>
      <div className="ph-note">// AGUARDANDO IMPLEMENTAÇÃO</div>
    </div>
  );
}

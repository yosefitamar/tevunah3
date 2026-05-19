"use client";

import AgentesScreen from "./agentes/AgentesScreen";
import AdminScreen from "./admin/AdminScreen";
import AprovacoesScreen from "./aprovacoes/AprovacoesScreen";
import AuditoriaScreen from "./auditoria/AuditoriaScreen";
import EntidadesScreen from "./entidades/EntidadesScreen";
import RelatoriosScreen from "./relatorios/RelatoriosScreen";

type PlaceholderProps = {
  tag: string;
  title: string;
  sub: string;
};

function Placeholder({ tag, title, sub }: PlaceholderProps) {
  return (
    <div className="placeholder">
      <div className="ph-tag">{tag}</div>
      <div className="ph-ttl">{title}</div>
      <div className="ph-sub">{sub}</div>
      <div className="ph-note">// MÓDULO EM IMPLEMENTAÇÃO</div>
    </div>
  );
}

export const ScreenAgentes = AgentesScreen;

export const ScreenEntidades = EntidadesScreen;

export const ScreenAprovacoes = AprovacoesScreen;

export const ScreenAuditoria = AuditoriaScreen;

export const ScreenAdmin = AdminScreen;

export const ScreenRelatorios = RelatoriosScreen;

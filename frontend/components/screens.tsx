"use client";

import AgentesScreen from "./agentes/AgentesScreen";
import AprovacoesScreen from "./aprovacoes/AprovacoesScreen";

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

export const ScreenAprovacoes = AprovacoesScreen;

export const ScreenAuditoria = () => (
  <Placeholder
    tag="// MOD-03 / AUDITORIA"
    title="TRILHO DE AUDITORIA"
    sub="Registro imutável de toda ação do sistema — logins, leituras, alterações, tentativas negadas. Append-only com cadeia de hash. Consultável por gestor e administrador."
  />
);

export const ScreenAdmin = () => (
  <Placeholder
    tag="// MOD-99 / ADMIN"
    title="CONFIGURAÇÕES DO SISTEMA"
    sub="Matriz de permissões (RBAC ajustável), parametrização de 4-eyes por ação, integrações e parâmetros gerais. Restrito ao administrador."
  />
);

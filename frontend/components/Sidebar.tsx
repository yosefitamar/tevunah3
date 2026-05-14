"use client";

import { NAV, type ModuleId } from "@/lib/nav";
import TevunahLogo from "./TevunahLogo";

function Brand({
  collapsed,
  onGoHome,
}: {
  collapsed: boolean;
  onGoHome: () => void;
}) {
  return (
    <button
      type="button"
      className="brand brand--clickable"
      onClick={onGoHome}
      title="Ir para o dashboard"
      aria-label="Ir para o dashboard"
    >
      <div className="brand-glyph">
        <TevunahLogo />
      </div>
      {!collapsed && (
        <div className="brand-text">
          <span className="brand-heb">תבונה</span>
        </div>
      )}
    </button>
  );
}

type Props = {
  active: ModuleId;
  setActive: (id: ModuleId) => void;
  collapsed: boolean;
  setCollapsed: (v: boolean) => void;
};

export default function Sidebar({ active, setActive, collapsed, setCollapsed }: Props) {
  return (
    <aside className={"sidebar" + (collapsed ? " collapsed" : "")}>
      <Brand collapsed={collapsed} onGoHome={() => setActive("dashboard")} />
      <div className="nav">
        {NAV.map((n, i) =>
          "group" in n ? (
            <div key={"g" + i} className="nav-section">
              <span>{n.group}</span>
            </div>
          ) : (
            <button
              key={n.id}
              className={"nav-item" + (active === n.id ? " active" : "")}
              onClick={() => setActive(n.id)}
              title={collapsed ? n.lbl : undefined}
              type="button"
            >
              <span className="key">{n.glyph}</span>
              <span className="lbl">{n.lbl}</span>
              {n.badge && (
                <span className={"badge" + (n.badgeKind ? " " + n.badgeKind : "")}>
                  {n.badge}
                </span>
              )}
            </button>
          )
        )}
      </div>
      <div className="sidebar-foot">
        <button
          className="collapse-btn"
          onClick={() => setCollapsed(!collapsed)}
          title={collapsed ? "Expandir" : "Recolher"}
          type="button"
        >
          {collapsed ? "›" : "‹"}
        </button>
        <span className="foot-text">v1.0.4 · BUILD 8841</span>
      </div>
    </aside>
  );
}

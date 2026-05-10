"use client";

const MONTHS_PT = ["JAN", "FEV", "MAR", "ABR", "MAI", "JUN", "JUL", "AGO", "SET", "OUT", "NOV", "DEZ"];
function intelDate(d: Date) {
  const dd = String(d.getDate()).padStart(2, "0");
  const mmm = MONTHS_PT[d.getMonth()];
  const yy = String(d.getFullYear()).slice(-2);
  return `${dd}${mmm}${yy}`;
}

export default function Dashboard() {
  const today = intelDate(new Date());
  return (
    <>
      <div className="section-title">
        PAINEL OPERACIONAL · {today}
        <span style={{ color: "var(--fg-2)" }}>· VISTA GERAL</span>
      </div>

      <div className="placeholder" style={{ minHeight: 280 }}>
        <div className="ph-tag">// MOD-01 / DASHBOARD</div>
        <div className="ph-ttl">PAINEL EM CONSTRUÇÃO</div>
        <div className="ph-sub">
          O painel será populado conforme novos módulos forem ativados. No estado atual o sistema
          contempla apenas o cadastro de agentes, a trilha de auditoria e a administração do
          sistema.
        </div>
        <div className="ph-note">// SEM DADOS OPERACIONAIS PARA EXIBIR</div>
      </div>
    </>
  );
}

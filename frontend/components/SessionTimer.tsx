"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";

/**
 * Timer regressivo da sessão. Lê `sessionExpiresAt` do AuthContext, que é
 * atualizado pelo header `X-Session-Expires-At` em cada chamada autenticada.
 * Atualiza a cada segundo. Muda de cor abaixo de 2 min (warn) e 1 min (crit).
 */
export default function SessionTimer() {
  const { sessionExpiresAt } = useAuth();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  if (!sessionExpiresAt) {
    return (
      <div className="stat" title="Sessão">
        <span className="lbl">SESSÃO</span>
        <span className="val muted">--:--</span>
      </div>
    );
  }

  const msLeft = Math.max(0, sessionExpiresAt.getTime() - now);
  const totalSec = Math.floor(msLeft / 1000);
  const mm = Math.floor(totalSec / 60).toString().padStart(2, "0");
  const ss = (totalSec % 60).toString().padStart(2, "0");

  let cls = "";
  if (totalSec < 60) cls = " session-timer--crit";
  else if (totalSec < 120) cls = " session-timer--warn";

  return (
    <div
      className={"stat session-timer" + cls}
      title={`Sessão expira às ${sessionExpiresAt.toLocaleTimeString("pt-BR")}`}
    >
      <span className="lbl">SESSÃO</span>
      <span className="val" style={{ fontFeatureSettings: '"tnum"' }}>
        {mm}:{ss}
      </span>
    </div>
  );
}

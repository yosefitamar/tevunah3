"use client";

import { useEffect, useRef } from "react";
import { api, type ApiError } from "@/lib/api";

/**
 * Keep-alive de sessão baseado em atividade real do agente.
 *
 * O TTL da sessão é idle-refreshed no servidor: só renova quando chega uma
 * requisição autenticada. Isso derruba quem está numa tela de trabalho longa
 * sem tráfego de rede — o caso clássico é o redator digitando um relatório por
 * 20 minutos. Aqui escutamos eventos que só existem com humano presente
 * (tecla, clique, colagem, arrastar arquivo) e, no máximo uma vez por minuto,
 * batemos em /api/auth/heartbeat pra renovar o TTL.
 *
 * O que NÃO renova, de propósito: mouse parado, aba em segundo plano, timer
 * cego. Sem atividade, a sessão continua expirando como antes — o objetivo de
 * proteger dados sensíveis em estação abandonada segue valendo.
 */

// Eventos contados como presença. `mousemove` fica de fora: mexer no mouse ao
// passar pela mesa não deveria segurar a sessão de quem saiu.
const ACTIVITY_EVENTS = [
  "keydown",
  "pointerdown",
  "input",
  "change",
  "paste",
  "drop",
] as const;

/** Intervalo mínimo entre heartbeats (também o passo de verificação). */
const HEARTBEAT_INTERVAL_MS = 60_000;
const CHECK_INTERVAL_MS = 15_000;

export function useSessionKeepAlive(active: boolean, onExpired?: () => void) {
  // Refs em vez de state: marcar atividade não pode re-renderizar a árvore
  // inteira a cada tecla digitada.
  const lastActivityRef = useRef(0);
  const lastPingRef = useRef(0);
  const inFlightRef = useRef(false);
  const onExpiredRef = useRef(onExpired);

  useEffect(() => {
    onExpiredRef.current = onExpired;
  }, [onExpired]);

  useEffect(() => {
    if (!active) return;

    // Começa do zero a cada ativação (login / re-auth pós-overlay).
    lastActivityRef.current = 0;
    lastPingRef.current = Date.now();

    const markActivity = () => {
      lastActivityRef.current = Date.now();
    };
    for (const ev of ACTIVITY_EVENTS) {
      window.addEventListener(ev, markActivity, { capture: true, passive: true });
    }

    const tick = async () => {
      const now = Date.now();
      if (inFlightRef.current) return;
      // Aba escondida não conta como agente na frente da tela.
      if (document.visibilityState !== "visible") return;
      // Só renova se houve atividade depois do último heartbeat.
      if (lastActivityRef.current <= lastPingRef.current) return;
      if (now - lastPingRef.current < HEARTBEAT_INTERVAL_MS) return;

      inFlightRef.current = true;
      lastPingRef.current = now;
      try {
        // O middleware renova o TTL, reemite o cookie e devolve
        // X-Session-Expires-At — o timer da topbar se atualiza sozinho.
        await api<void>("/api/auth/heartbeat", { method: "POST" });
      } catch (e) {
        // /api/auth/* não dispara o handler global de 401 (por causa do
        // login), então o overlay de re-auth é avisado aqui.
        if ((e as ApiError).status === 401) onExpiredRef.current?.();
        // Demais erros (rede/5xx) só ignoram: o próximo tick tenta de novo.
      } finally {
        inFlightRef.current = false;
      }
    };

    const id = window.setInterval(tick, CHECK_INTERVAL_MS);
    return () => {
      window.clearInterval(id);
      for (const ev of ACTIVITY_EVENTS) {
        window.removeEventListener(ev, markActivity, { capture: true });
      }
    };
  }, [active]);
}

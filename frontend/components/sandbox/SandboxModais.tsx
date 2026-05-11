"use client";

import { useState } from "react";
import { useModal } from "@/contexts/ModalContext";

export default function SandboxModais() {
  const modal = useModal();
  const [lastResult, setLastResult] = useState<string>("—");

  // ─── Loading ─────────────────────────────────────────────────────
  async function loadingDefault() {
    const h = modal.loading({ message: "PROCESSANDO…" });
    await sleep(2000);
    h.close();
    setLastResult("loading default: encerrado após 2s");
  }
  async function loadingFast() {
    const h = modal.loading({ message: "Não deveria aparecer." });
    await sleep(300);
    h.close();
    setLastResult("loading fast (300ms): NÃO mostrou modal (threshold 800ms)");
  }
  async function loadingThresholdZero() {
    const h = modal.loading({ message: "VAI APARECER NA HORA", thresholdMs: 0 });
    await sleep(1500);
    h.close();
    setLastResult("loading threshold=0: apareceu imediatamente");
  }
  async function loadingCustomThreshold() {
    const h = modal.loading({ message: "SINCRONIZANDO COM SERVIDOR…", thresholdMs: 200 });
    await sleep(2500);
    h.close();
    setLastResult("loading com threshold=200ms");
  }

  // ─── Alert ───────────────────────────────────────────────────────
  async function alertInfo() {
    await modal.alert({
      variant: "info",
      title: "INFORMAÇÃO",
      message:
        "Esta é uma mensagem informativa neutra. Clique em OK para fechar, ou pressione Esc, ou clique fora do modal.",
    });
    setLastResult("alert info: fechado");
  }
  async function alertSuccess() {
    await modal.alert({
      variant: "success",
      title: "OPERAÇÃO BEM-SUCEDIDA",
      message: "Permissão atualizada com sucesso. Este modal auto-fecha em 1.5s.",
    });
    setLastResult("alert success: auto-fechou");
  }
  async function alertWarning() {
    await modal.alert({
      variant: "warning",
      title: "ATENÇÃO",
      message:
        "Você está prestes a sair sem salvar. Esta ação é reversível, mas as alterações serão perdidas.",
    });
    setLastResult("alert warning: fechado");
  }
  async function alertError() {
    await modal.alert({
      variant: "error",
      title: "FALHA NA OPERAÇÃO",
      message:
        "Não foi possível conectar ao servidor. Verifique sua conexão e tente novamente. Código: NET_TIMEOUT.",
    });
    setLastResult("alert error: fechado");
  }
  async function alertSemTitulo() {
    await modal.alert({
      variant: "info",
      message: "Mensagem sem título. Útil para avisos curtos.",
    });
    setLastResult("alert info (sem título): fechado");
  }
  async function alertCustomOk() {
    await modal.alert({
      variant: "warning",
      title: "ATUALIZAÇÃO DISPONÍVEL",
      message: "Uma nova versão do sistema está pronta.",
      ok: "ENTENDIDO",
    });
    setLastResult("alert com botão customizado");
  }

  // ─── Confirm ─────────────────────────────────────────────────────
  async function confirmNormal() {
    const ok = await modal.confirm({
      variant: "warning",
      title: "REINICIAR SESSÃO?",
      message:
        "Você será redirecionado para a tela de login e precisará informar novamente suas credenciais e TOTP.",
    });
    setLastResult(`confirm normal: ${ok ? "CONFIRMOU" : "CANCELOU"}`);
  }
  async function confirmDanger() {
    const ok = await modal.confirm({
      variant: "error",
      title: "DESATIVAR AGENTE?",
      message:
        "Esta ação revoga todas as sessões ativas do agente e fica registrada no audit log. O agente pode ser reativado posteriormente.",
      confirm: "DESATIVAR",
      cancel: "MANTER",
      danger: true,
    });
    setLastResult(`confirm danger: ${ok ? "DESATIVOU" : "MANTEVE"}`);
  }
  async function confirmInfo() {
    const ok = await modal.confirm({
      variant: "info",
      title: "EXPORTAR DADOS?",
      message: "Vamos gerar um CSV com 247 registros filtrados.",
      confirm: "EXPORTAR",
      cancel: "AGORA NÃO",
    });
    setLastResult(`confirm info: ${ok ? "EXPORTAR" : "AGORA NÃO"}`);
  }

  // ─── Composto / stack ────────────────────────────────────────────
  async function confirmDepoisLoadingDepoisSuccess() {
    const ok = await modal.confirm({
      variant: "warning",
      title: "EXECUTAR OPERAÇÃO DEMORADA?",
      message:
        "Vai disparar um confirm → loading → success. Demonstra o fluxo completo de uma ação que muda estado.",
      confirm: "EXECUTAR",
      cancel: "CANCELAR",
    });
    if (!ok) {
      setLastResult("composto: cancelado no confirm");
      return;
    }
    const h = modal.loading({ message: "EXECUTANDO OPERAÇÃO…" });
    await sleep(1800);
    h.close();
    await modal.alert({
      variant: "success",
      title: "PRONTO",
      message: "Operação concluída em 1.8s.",
    });
    setLastResult("composto: confirm + loading + success ✓");
  }
  async function stackDuplo() {
    // Abre dois alerts em sequência sem fechar o primeiro — só para ver
    // visualmente o card de baixo escurecido pelo de cima.
    modal.alert({
      variant: "info",
      title: "PRIMEIRO MODAL",
      message:
        "Em 600ms outro modal vai abrir por cima deste. Observe que este aqui escurece e fica não-interativo.",
    });
    setTimeout(() => {
      modal.alert({
        variant: "warning",
        title: "SEGUNDO MODAL",
        message: "Estou no topo da pilha. Feche-me primeiro.",
      });
    }, 600);
    setLastResult("stack duplo: dois alerts empilhados");
  }
  async function loadingDuranteAlert() {
    const h = modal.loading({ message: "ASSÍNCRONO EM BACKGROUND…", thresholdMs: 0 });
    await modal.alert({
      variant: "info",
      title: "INTERAÇÃO COM LOADING ATIVO",
      message: "Há um loading rodando atrás. Feche este para continuar.",
    });
    await sleep(1200);
    h.close();
    setLastResult("loading + alert simultâneos");
  }

  return (
    <div className="screen-fill">
      <div className="section-title">
        SANDBOX · MODAIS
        <span style={{ color: "var(--fg-2)" }}>· DEV-ONLY · TRIGGERS DE TODAS AS VARIANTES</span>
      </div>

      <div className="banner banner-info" style={{ marginBottom: 16 }}>
        ⌬ Página temporária para inspecionar visualmente o sistema de modais. Cada
        botão dispara uma chamada à API <code>useModal()</code>. O resultado do
        último teste aparece em destaque abaixo.
      </div>

      <div className="sandbox-result">
        <div className="sandbox-result-lbl">// ÚLTIMO RESULTADO</div>
        <div className="sandbox-result-val">{lastResult}</div>
      </div>

      <Section title="LOADING" hint="Threshold default = 800ms. Operações mais rápidas não mostram modal.">
        <TestBtn onClick={loadingDefault}>LOADING 2s (default 800ms)</TestBtn>
        <TestBtn onClick={loadingFast}>LOADING 300ms (não deve aparecer)</TestBtn>
        <TestBtn onClick={loadingThresholdZero}>LOADING threshold=0 · 1.5s</TestBtn>
        <TestBtn onClick={loadingCustomThreshold}>LOADING threshold=200ms · 2.5s</TestBtn>
      </Section>

      <Section title="ALERT" hint="Mensagem com OK. Variantes mudam cor e ícone.">
        <TestBtn onClick={alertInfo}>ALERT info</TestBtn>
        <TestBtn onClick={alertSuccess}>ALERT success (auto-fecha)</TestBtn>
        <TestBtn onClick={alertWarning}>ALERT warning</TestBtn>
        <TestBtn onClick={alertError}>ALERT error</TestBtn>
        <TestBtn onClick={alertSemTitulo}>ALERT sem título</TestBtn>
        <TestBtn onClick={alertCustomOk}>ALERT com botão customizado</TestBtn>
      </Section>

      <Section title="CONFIRM" hint="Dois botões. Retorna boolean.">
        <TestBtn onClick={confirmNormal}>CONFIRM warning</TestBtn>
        <TestBtn onClick={confirmDanger}>CONFIRM error + danger</TestBtn>
        <TestBtn onClick={confirmInfo}>CONFIRM info (com labels)</TestBtn>
      </Section>

      <Section title="COMPOSTOS · STACK" hint="Fluxos com mais de um modal.">
        <TestBtn onClick={confirmDepoisLoadingDepoisSuccess}>
          CONFIRM → LOADING → SUCCESS
        </TestBtn>
        <TestBtn onClick={stackDuplo}>STACK · 2 ALERTS EMPILHADOS</TestBtn>
        <TestBtn onClick={loadingDuranteAlert}>LOADING + ALERT SIMULTÂNEOS</TestBtn>
      </Section>
    </div>
  );
}

function sleep(ms: number) {
  return new Promise<void>((r) => window.setTimeout(r, ms));
}

function Section({
  title,
  hint,
  children,
}: {
  title: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="sandbox-section">
      <div className="sandbox-section-hd">
        <span className="sandbox-section-title">▸ {title}</span>
        {hint && <span className="sandbox-section-hint">{hint}</span>}
      </div>
      <div className="sandbox-btns">{children}</div>
    </div>
  );
}

function TestBtn({
  onClick,
  children,
}: {
  onClick: () => void | Promise<void>;
  children: React.ReactNode;
}) {
  return (
    <button type="button" className="btn" onClick={onClick}>
      {children}
    </button>
  );
}

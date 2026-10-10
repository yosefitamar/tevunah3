"use client";

import type { ReactNode } from "react";

/**
 * Peças de layout da ficha da ocorrência do relatório operacional. Tudo na
 * ficha — o que o PDF trouxe, a tradução para o SIPOM, os controles de
 * correção — cai na mesma grade de 4 colunas, com rótulo em cima e valor (ou
 * controle) embaixo, para as linhas ficarem alinhadas entre os blocos.
 */

/** Bloco de uma etapa: título à esquerda, ação opcional à direita. */
export function OpsBlock({
  title,
  aside,
  children,
}: {
  title: ReactNode;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="ops-block">
      <div className="ops-block-title">
        <span className="ops-block-title-txt">{title}</span>
        {aside}
      </div>
      {children}
    </div>
  );
}

/** Grade de campos. `edit`: há controles na grade — os valores só de leitura
 *  ganham a altura do controle para a linha não desalinhar. */
export function OpsFields({ edit = false, children }: { edit?: boolean; children: ReactNode }) {
  return <dl className={"ops-fields" + (edit ? " ops-fields--edit" : "")}>{children}</dl>;
}

export function OpsField({
  label,
  value,
  span = 1,
  missing = false,
  tag,
  action,
  mono = false,
  children,
}: {
  label: ReactNode;
  value?: ReactNode;
  /** Colunas ocupadas na grade de 4. */
  span?: 1 | 2 | 3 | 4;
  /** Campo que ainda impede o envio ao SIPOM: fica em âmbar. */
  missing?: boolean;
  /** Origem do valor (ANALISTA, REFERÊNCIA…). */
  tag?: string;
  action?: ReactNode;
  mono?: boolean;
  /** Controle de edição; quando presente, substitui o valor. */
  children?: ReactNode;
}) {
  return (
    <div className={"ops-field ops-field--s" + span + (missing ? " ops-field--missing" : "")}>
      <dt>
        <span className="ops-field-lbl">{label}</span>
        {tag && <span className="sipom-tag">{tag}</span>}
        {action}
      </dt>
      {/* `||`: o filho condicional chega como `false` quando o campo não é
          editável, e aí vale o valor. */}
      <dd className={mono ? "mono" : undefined}>{children || value || "—"}</dd>
    </div>
  );
}

/** Linha de botões alinhada à grade (ocupa a largura toda). */
export function OpsActions({ children }: { children: ReactNode }) {
  return <div className="ops-actions">{children}</div>;
}

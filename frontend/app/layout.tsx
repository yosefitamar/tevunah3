import type { Metadata } from "next";
import { UI_SCALE_BOOTSTRAP } from "@/lib/ui-scale";
import "./globals.css";

export const metadata: Metadata = {
  title: "Tevunah · Sistema de Inteligência",
  description: "Belia Tevunah — Sistema de Inteligência",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // suppressHydrationWarning: o script abaixo escreve style no <html> antes
    // do React hidratar, então o atributo do cliente diverge do servidor por
    // construção. O aviso vale só para este elemento, não para a árvore.
    <html lang="pt-BR" data-palette="phosphor" suppressHydrationWarning>
      <head>
        {/* Escala da interface antes da primeira pintura — sem isto a página
            nasce em 100% e salta para a escala do agente na hidratação. */}
        <script dangerouslySetInnerHTML={{ __html: UI_SCALE_BOOTSTRAP }} />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&family=Noto+Sans+Hebrew:wght@400;500;700;900&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>{children}</body>
    </html>
  );
}

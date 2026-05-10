import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Tevunah · Sistema de Inteligência",
  description: "Belia Tevunah — Sistema de Inteligência",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="pt-BR" data-palette="phosphor">
      <head>
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

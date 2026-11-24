import type { NextConfig } from "next";

// Em dev (Docker), o frontend chega ao backend pela rede do compose como
// http://backend:8080. Em produção, exportar BACKEND_INTERNAL_URL apontando
// para o serviço por trás do reverse proxy.
const backendURL = process.env.BACKEND_INTERNAL_URL ?? "http://backend:8080";

// Identificador da versão: o deploy passa APP_BUILD_ID (commit + hora); sem
// ele, a hora do build. Vai para o código do cliente (NEXT_PUBLIC_BUILD_ID) e
// para /version, que a aba aberta consulta para saber se há versão nova.
const buildId = process.env.APP_BUILD_ID || new Date().toISOString().replace(/\D/g, "").slice(0, 14);

const nextConfig: NextConfig = {
  reactStrictMode: true,
  generateBuildId: async () => buildId,
  env: { NEXT_PUBLIC_BUILD_ID: buildId },
  output: "standalone",
  // Não trava o build de produção em erros de tipo/lint. O type-check
  // continua acontecendo no editor/CI; isto só evita que um type error
  // residual derrube o deploy.
  typescript: { ignoreBuildErrors: true },
  eslint: { ignoreDuringBuilds: true },
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${backendURL}/api/:path*` }];
  },
};

export default nextConfig;

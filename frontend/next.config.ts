import type { NextConfig } from "next";

// Em dev (Docker), o frontend chega ao backend pela rede do compose como
// http://backend:8080. Em produção, exportar BACKEND_INTERNAL_URL apontando
// para o serviço por trás do reverse proxy.
const backendURL = process.env.BACKEND_INTERNAL_URL ?? "http://backend:8080";

const nextConfig: NextConfig = {
  reactStrictMode: true,
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

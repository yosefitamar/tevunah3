FROM node:22-slim AS deps
WORKDIR /app
# Não copiamos o package-lock.json de propósito: ele é gerado em darwin e
# omite os binários nativos linux opcionais (lightningcss, @tailwindcss/oxide),
# causando "Cannot find module lightningcss.linux-x64-gnu.node" no build.
# Sem o lockfile, npm resolve os optionals corretamente pra plataforma alvo.
COPY package.json ./
RUN npm install --no-audit --no-fund

FROM node:22-slim AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-slim AS runner
WORKDIR /app

ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

RUN groupadd --system --gid 1001 nodejs \
 && useradd  --system --uid 1001 --gid nodejs nextjs

COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000

CMD ["node", "server.js"]

#!/usr/bin/env bash
# Tevunah — instalação/deploy nativo em Debian 12.
#
# Idempotente: pode rodar de novo a qualquer momento para atualizar
# binários (após `git pull`) e reiniciar os serviços. Não destrói dados.
#
# Uso (como root):
#   cd /opt/tevunah
#   bash deploy/install.sh
#
# Pré-requisitos:
#   - Debian 12 (bookworm)
#   - /opt/tevunah/.env.prod preenchido (cp .env.prod.example .env.prod)
#   - Conexão de internet

set -euo pipefail

REPO_DIR="/opt/tevunah"
APP_USER="tevunah"
GO_VERSION="1.23.4"
NODE_MAJOR="22"
PHOTO_DIR="/var/lib/tevunah/photos"
WKHTML_DEB_URL="https://github.com/wkhtmltopdf/packaging/releases/download/0.12.6.1-3/wkhtmltox_0.12.6.1-3.bookworm_amd64.deb"

log()  { printf "\n\033[1;36m▶ %s\033[0m\n" "$*"; }
warn() { printf "\033[1;33m⚠ %s\033[0m\n" "$*"; }
fail() { printf "\033[1;31m✗ %s\033[0m\n" "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || fail "Rode como root (sudo bash deploy/install.sh)"
[[ -d "$REPO_DIR" ]] || fail "Repositório esperado em $REPO_DIR"
cd "$REPO_DIR"

# ─── 1. Variáveis de ambiente ────────────────────────────────────────────
log "Carregando .env.prod"
[[ -f .env.prod ]] || fail ".env.prod não existe. Copie do .env.prod.example e ajuste."
set -a; . ./.env.prod; set +a

for var in POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB \
           APP_DB_USER APP_DB_PASSWORD AUDIT_DB_USER AUDIT_DB_PASSWORD; do
  [[ -n "${!var:-}" ]] || fail ".env.prod: $var vazio"
  [[ "${!var}" != CHANGE_ME* ]] || fail ".env.prod: $var ainda está com placeholder CHANGE_ME_*"
done

SESSION_IDLE_MINUTES="${SESSION_IDLE_MINUTES:-15}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

# ─── 2. Pacotes de sistema ───────────────────────────────────────────────
log "Instalando pacotes apt"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq \
  ca-certificates curl wget gnupg git build-essential \
  postgresql-15 postgresql-client-15 \
  redis-server \
  libxrender1 libxext6 libfontconfig1 libjpeg62-turbo xfonts-base xfonts-75dpi \
  fontconfig

# ─── 3. wkhtmltopdf (versão com Qt patched, do upstream) ─────────────────
if ! command -v wkhtmltopdf >/dev/null 2>&1 || ! wkhtmltopdf --version 2>/dev/null | grep -q "with patched qt"; then
  log "Instalando wkhtmltopdf (patched Qt) do upstream"
  TMPDEB=$(mktemp --suffix=.deb)
  curl -fsSL -o "$TMPDEB" "$WKHTML_DEB_URL"
  apt-get install -y -qq "$TMPDEB"
  rm -f "$TMPDEB"
else
  log "wkhtmltopdf já instalado: $(wkhtmltopdf --version)"
fi

# ─── 4. Go ────────────────────────────────────────────────────────────────
INSTALLED_GO=""
if command -v /usr/local/go/bin/go >/dev/null 2>&1; then
  INSTALLED_GO=$(/usr/local/go/bin/go version | awk '{print $3}' | sed 's/go//')
fi
if [[ "$INSTALLED_GO" != "$GO_VERSION" ]]; then
  log "Instalando Go $GO_VERSION"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  rm /tmp/go.tgz
fi
export PATH="/usr/local/go/bin:$PATH"
log "Go: $(go version)"

# ─── 5. Node.js ──────────────────────────────────────────────────────────
INSTALLED_NODE_MAJOR=""
if command -v node >/dev/null 2>&1; then
  INSTALLED_NODE_MAJOR=$(node -v | sed 's/v//' | cut -d. -f1)
fi
if [[ "$INSTALLED_NODE_MAJOR" != "$NODE_MAJOR" ]]; then
  log "Instalando Node.js $NODE_MAJOR.x (NodeSource)"
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get install -y -qq nodejs
fi
log "Node: $(node -v)"

# ─── 6. Usuário do app ───────────────────────────────────────────────────
if ! id -u "$APP_USER" >/dev/null 2>&1; then
  log "Criando usuário $APP_USER"
  adduser --system --group --home /home/$APP_USER --shell /bin/bash "$APP_USER"
fi
mkdir -p "$PHOTO_DIR"
chown -R "$APP_USER:$APP_USER" "$PHOTO_DIR" "$REPO_DIR"

# ─── 7. PostgreSQL: serviço, roles, db ───────────────────────────────────
log "Configurando PostgreSQL"
systemctl enable --now postgresql

# Roda como o usuário postgres
PG_SQL=$(cat <<EOSQL
DO \$\$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${POSTGRES_USER}') THEN
    CREATE ROLE ${POSTGRES_USER} WITH LOGIN SUPERUSER PASSWORD '${POSTGRES_PASSWORD}';
  ELSE
    ALTER ROLE ${POSTGRES_USER} WITH LOGIN SUPERUSER PASSWORD '${POSTGRES_PASSWORD}';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${APP_DB_USER}') THEN
    CREATE ROLE ${APP_DB_USER} WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';
  ELSE
    ALTER ROLE ${APP_DB_USER} WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${AUDIT_DB_USER}') THEN
    CREATE ROLE ${AUDIT_DB_USER} WITH LOGIN PASSWORD '${AUDIT_DB_PASSWORD}';
  ELSE
    ALTER ROLE ${AUDIT_DB_USER} WITH LOGIN PASSWORD '${AUDIT_DB_PASSWORD}';
  END IF;
END \$\$;
EOSQL
)
sudo -u postgres psql -v ON_ERROR_STOP=1 -c "$PG_SQL"

# Cria o database se ainda não existe
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='${POSTGRES_DB}'" | grep -q 1; then
  sudo -u postgres createdb -O "${POSTGRES_USER}" "${POSTGRES_DB}"
fi

sudo -u postgres psql -v ON_ERROR_STOP=1 -d "${POSTGRES_DB}" <<EOSQL
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${APP_DB_USER};
GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${AUDIT_DB_USER};
EOSQL

# ─── 8. Redis ────────────────────────────────────────────────────────────
log "Habilitando Redis"
systemctl enable --now redis-server

# ─── 9. Build do backend ─────────────────────────────────────────────────
log "Compilando binários Go"
mkdir -p "$REPO_DIR/bin"
sudo -u "$APP_USER" -H bash -c "
  set -e
  export PATH='/usr/local/go/bin:\$PATH'
  export GOCACHE='$REPO_DIR/.gocache'
  export GOPATH='$REPO_DIR/.gopath'
  cd '$REPO_DIR/backend'
  go build -trimpath -ldflags='-s -w' -o '$REPO_DIR/bin/server'  ./cmd/server
  go build -trimpath -ldflags='-s -w' -o '$REPO_DIR/bin/migrate' ./cmd/migrate
  go build -trimpath -ldflags='-s -w' -o '$REPO_DIR/bin/admin'   ./cmd/admin
"

# ─── 10. Build do frontend (Next.js standalone) ──────────────────────────
log "Build do frontend (Next.js)"
sudo -u "$APP_USER" -H bash -c "
  set -e
  cd '$REPO_DIR/frontend'
  # lockfile gerado em darwin não tem binários nativos linux opcionais
  # (lightningcss / @tailwindcss/oxide); deletamos para resolução fresca.
  rm -f package-lock.json
  npm install --no-audit --no-fund
  npm run build
"

# Copia o standalone output pra um diretório estável
rm -rf "$REPO_DIR/frontend-build"
mkdir -p "$REPO_DIR/frontend-build"
cp -a "$REPO_DIR/frontend/.next/standalone/." "$REPO_DIR/frontend-build/"
cp -a "$REPO_DIR/frontend/.next/static"       "$REPO_DIR/frontend-build/.next/static"
chown -R "$APP_USER:$APP_USER" "$REPO_DIR/frontend-build"

# ─── 11. Arquivos de env do systemd ──────────────────────────────────────
log "Escrevendo backend.env / frontend.env"
cat > "$REPO_DIR/backend.env" <<EOF
APP_ENV=production
ADDR=:8080
PHOTO_DIR=$PHOTO_DIR
MIGRATIONS_DATABASE_URL=postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable
APP_DATABASE_URL=postgres://${APP_DB_USER}:${APP_DB_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable
AUDIT_DATABASE_URL=postgres://${AUDIT_DB_USER}:${AUDIT_DB_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379/0
SESSION_IDLE_MINUTES=${SESSION_IDLE_MINUTES}
EOF

cat > "$REPO_DIR/frontend.env" <<EOF
NODE_ENV=production
NEXT_TELEMETRY_DISABLED=1
PORT=${FRONTEND_PORT}
HOSTNAME=0.0.0.0
BACKEND_INTERNAL_URL=http://127.0.0.1:8080
EOF

chmod 600 "$REPO_DIR/backend.env" "$REPO_DIR/frontend.env"
chown "$APP_USER:$APP_USER" "$REPO_DIR/backend.env" "$REPO_DIR/frontend.env"

# ─── 12. Migrations ──────────────────────────────────────────────────────
log "Aplicando migrations"
sudo -u "$APP_USER" -H bash -c "
  set -a; . '$REPO_DIR/backend.env'; set +a
  '$REPO_DIR/bin/migrate' up
"

# ─── 13. systemd units ───────────────────────────────────────────────────
log "Instalando units do systemd"
install -m 644 deploy/systemd/tevunah-backend.service  /etc/systemd/system/
install -m 644 deploy/systemd/tevunah-frontend.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable tevunah-backend tevunah-frontend
systemctl restart tevunah-backend
systemctl restart tevunah-frontend

# ─── 14. Resumo ──────────────────────────────────────────────────────────
sleep 2
log "Status dos serviços"
systemctl --no-pager --lines=0 status tevunah-backend  | head -3 || true
systemctl --no-pager --lines=0 status tevunah-frontend | head -3 || true

IP=$(ip -4 addr show scope global | awk '/inet /{print $2}' | head -1 | cut -d/ -f1)
cat <<EOF

✓ Deploy concluído.

  Backend:  systemctl status tevunah-backend
            journalctl -u tevunah-backend -f
  Frontend: systemctl status tevunah-frontend
            journalctl -u tevunah-frontend -f

  Acesso interno:  http://${IP}:${FRONTEND_PORT}

Próximo:
  • Criar o primeiro admin:
      sudo -u $APP_USER -H bash -c 'set -a; . $REPO_DIR/backend.env; set +a; $REPO_DIR/bin/admin create'
  • Adicionar ingress rule no cloudflared apontando pra http://${IP}:${FRONTEND_PORT}

Pra atualizar (após git pull): rode novamente este script.
EOF

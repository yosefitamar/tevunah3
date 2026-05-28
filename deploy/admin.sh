#!/usr/bin/env bash
# Wrapper para a CLI admin do Tevunah em produção.
# Carrega a env, troca pro user tevunah e executa o binário.
#
# Uso (como root):
#   bash deploy/admin.sh create        # cria admin interativamente
#   bash deploy/admin.sh seed-dev      # só em APP_ENV=development

set -euo pipefail
REPO_DIR="/opt/tevunah"
APP_USER="tevunah"

[[ $EUID -eq 0 ]] || { echo "Rode como root."; exit 1; }
[[ -f "$REPO_DIR/backend.env" ]] || { echo "backend.env não existe — rode deploy/install.sh primeiro."; exit 1; }
[[ -x "$REPO_DIR/bin/admin" ]] || { echo "binário admin não existe — rode deploy/install.sh primeiro."; exit 1; }

sudo -u "$APP_USER" -H bash -c "
  set -a; . '$REPO_DIR/backend.env'; set +a
  '$REPO_DIR/bin/admin' $*
"

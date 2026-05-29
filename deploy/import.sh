#!/usr/bin/env bash
# Wrapper para a importação do legado do Tevunah em produção (sem Docker).
# Carrega a env, restaura o schema legacy.* no Postgres e roda o transformador
# Go (legacy.* → app.*) como o user tevunah, copiando as mídias pro PHOTO_DIR.
#
# Pré-requisito: o schema legacy.* já convertido de MySQL → Postgres, gerado no
# ambiente de dev com `./tevunah import:load-mysql <dump.sql>` seguido de
# `pg_dump --schema=legacy --no-owner --no-privileges > legacy-schema.sql`.
#
# Layout do bundle:
#   <bundle>/
#     legacy-schema.sql   (pg_dump do schema legacy; restaurado antes de importar)
#     media/              (espelho de storage/app/public/ do Laravel)
#     pdfs/               (PDFs originais dos RIs, opcional)
#
# Uso (como root):
#   bash deploy/import.sh <bundle-dir>               # restaura legacy.* + transforma + copia mídias
#   bash deploy/import.sh --media-only <bundle-dir>  # só sincroniza mídias (entidades já importadas)

set -euo pipefail
REPO_DIR="/opt/tevunah"
APP_USER="tevunah"

media_only=0
if [[ "${1:-}" == "--media-only" ]]; then
  media_only=1
  shift
fi
bundle="${1:-}"

[[ $EUID -eq 0 ]] || { echo "Rode como root."; exit 1; }
[[ -n "$bundle" && -d "$bundle" ]] || { echo "Uso: bash deploy/import.sh [--media-only] <bundle-dir>"; exit 1; }
[[ -f "$REPO_DIR/backend.env" ]] || { echo "backend.env não existe — rode deploy/install.sh primeiro."; exit 1; }
[[ -x "$REPO_DIR/bin/import-legacy" ]] || { echo "binário import-legacy não existe — rode deploy/install.sh primeiro."; exit 1; }
[[ -d "$bundle/media" ]] || { echo "$bundle/media/ não encontrado."; exit 1; }

bundle_abs="$(cd "$bundle" && pwd)"

# O importer roda como $APP_USER; garante que ele consegue ler o bundle inteiro.
chown -R "$APP_USER:$APP_USER" "$bundle_abs"

if [[ "$media_only" -eq 0 ]]; then
  schema_sql="$bundle_abs/legacy-schema.sql"
  [[ -f "$schema_sql" ]] || { echo "$schema_sql não encontrado (gere no dev com pg_dump --schema=legacy)."; exit 1; }
  echo "▶ Restaurando schema legacy.* no Postgres…"
  sudo -u "$APP_USER" -H bash -c "
    set -euo pipefail
    set -a; . '$REPO_DIR/backend.env'; set +a
    psql \"\$MIGRATIONS_DATABASE_URL\" -c 'DROP SCHEMA IF EXISTS legacy CASCADE;'
    psql \"\$MIGRATIONS_DATABASE_URL\" -v ON_ERROR_STOP=1 -f '$schema_sql'
    n=\$(psql \"\$MIGRATIONS_DATABASE_URL\" -Atc 'SELECT count(*) FROM legacy.users')
    echo \"  legacy.users = \$n linha(s)\"
    [[ -n \"\$n\" && \"\$n\" != \"0\" ]] || { echo '❌ legacy.users vazio após o restore — verifique o legacy-schema.sql'; exit 1; }
  "
fi

extra_env=""
[[ "$media_only" -eq 1 ]] && extra_env="export IMPORT_MEDIA_ONLY=1"

echo "▶ Rodando transformador (legacy.* → app.*)…"
sudo -u "$APP_USER" -H bash -c "
  set -a; . '$REPO_DIR/backend.env'; set +a
  export LEGACY_BUNDLE_PATH='$bundle_abs'
  $extra_env
  '$REPO_DIR/bin/import-legacy'
"

echo ""
echo "✓ Importação concluída. Confira o último run:"
echo "  sudo -u postgres psql -d tevunah -c \"SELECT started_at, finished_at, stats FROM import.import_runs ORDER BY started_at DESC LIMIT 1;\""

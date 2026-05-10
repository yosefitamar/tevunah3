#!/bin/bash
# Cria os roles do Postgres usados pelo app e pelo audit writer.
# Roda apenas na primeira inicialização do volume do Postgres
# (docker-entrypoint executa /docker-entrypoint-initdb.d/* em ordem).

set -e

: "${APP_DB_USER:?APP_DB_USER não definido}"
: "${APP_DB_PASSWORD:?APP_DB_PASSWORD não definido}"
: "${AUDIT_DB_USER:?AUDIT_DB_USER não definido}"
: "${AUDIT_DB_PASSWORD:?AUDIT_DB_PASSWORD não definido}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  DO \$\$
  BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${APP_DB_USER}') THEN
      CREATE ROLE ${APP_DB_USER} WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';
    END IF;
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '${AUDIT_DB_USER}') THEN
      CREATE ROLE ${AUDIT_DB_USER} WITH LOGIN PASSWORD '${AUDIT_DB_PASSWORD}';
    END IF;
  END
  \$\$;

  GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${APP_DB_USER};
  GRANT CONNECT ON DATABASE ${POSTGRES_DB} TO ${AUDIT_DB_USER};
EOSQL

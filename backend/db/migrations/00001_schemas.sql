-- +goose Up
-- +goose StatementBegin

CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS audit;

-- Não há grants aqui; tabelas + grants em migrations subsequentes.
-- Roles tevunah_app e tevunah_audit_writer são criados pelo init script
-- 01_roles.sh do diretório /docker-entrypoint-initdb.d na primeira
-- inicialização do Postgres (ver backend/db/init/01_roles.sh).

GRANT USAGE ON SCHEMA app   TO tevunah_app;
GRANT USAGE ON SCHEMA audit TO tevunah_app;
GRANT USAGE ON SCHEMA audit TO tevunah_audit_writer;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA IF EXISTS audit CASCADE;
DROP SCHEMA IF EXISTS app   CASCADE;
-- +goose StatementEnd

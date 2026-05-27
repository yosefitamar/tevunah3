-- +goose Up
-- +goose StatementBegin

-- Schema dedicado para artefatos de importação do sistema legado (Laravel).
-- Separado de app.* pra deixar claro que: (a) é staging/auditoria, não
-- domínio; (b) pode ser droppado quando a migração estiver consolidada e
-- todos os mapeamentos forem irrelevantes.
CREATE SCHEMA IF NOT EXISTS import;

GRANT USAGE ON SCHEMA import TO tevunah_app;

-- import_runs: cada execução do comando ./tevunah import:legacy cria uma
-- linha aqui. Permite agrupar/auditar lotes e fazer "undo lógico" (não
-- físico — somos append-only) marcando registros derivados.
CREATE TABLE import.import_runs (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source       text NOT NULL,             -- "tevunah-legacy" (slug do sistema de origem)
  bundle_path  text NOT NULL,             -- diretório do bundle, pra rastrear o input
  started_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz NULL,
  triggered_by uuid NOT NULL REFERENCES app.users(id),
  stats        jsonb NOT NULL DEFAULT '{}'::jsonb  -- contadores finais (entidades, reports, anexos, órfãos…)
);

-- import_map: mapeamento legacy_id → uuid novo. Idempotência: ao re-rodar,
-- consultamos esta tabela antes de inserir; se já existe, reutilizamos o
-- uuid (e atualizamos campos se a fonte mudou). Garante que re-execuções
-- após correções de dados não duplicam registros nem perdem associações.
CREATE TABLE import.import_map (
  source        text NOT NULL,            -- "tevunah-legacy"
  legacy_table  text NOT NULL,            -- "users" | "suspects" | "internal_reports" | …
  legacy_id     bigint NOT NULL,
  new_uuid      uuid NOT NULL,
  run_id        uuid NOT NULL REFERENCES import.import_runs(id),
  imported_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (source, legacy_table, legacy_id)
);

CREATE INDEX import_map_new_uuid_idx ON import.import_map (new_uuid);

-- Usuário sintético "sistema-import": vira o created_by/updated_by de
-- qualquer registro cujo autor original (generated_by_user_id no legado)
-- não pôde ser mapeado — e também é o uploaded_by dos anexos importados.
-- Marcado como inativo pra não aparecer em listas/lookups operacionais.
INSERT INTO app.users (
  code, email, display_name, password_hash,
  status, clearance_level
) VALUES (
  'SYS-IMPORT',
  'sistema-import@tevunah.local',
  'SISTEMA · IMPORTAÇÃO LEGADO',
  -- argon2id de senha aleatória descartada — status='deactivated' já bloqueia
  -- login (auth checa IsActive antes de Verify); o hash existe só pra
  -- satisfazer o NOT NULL da coluna.
  '$argon2id$v=19$m=65536,t=3,p=4$ZGlzY2FyZGVkX3NhbHQwMA$/cVqJ87bzZpEgvgKQk72yT3kU8VmZJqGqz0WQqLrQF0',
  'deactivated',
  1
)
ON CONFLICT (email) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.users WHERE code = 'SYS-IMPORT';
DROP TABLE IF EXISTS import.import_map;
DROP TABLE IF EXISTS import.import_runs;
DROP SCHEMA IF EXISTS import;
-- +goose StatementEnd

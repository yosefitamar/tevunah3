-- +goose Up
-- +goose StatementBegin

-- ─── Meio utilizado (CVLI) ────────────────────────────────────────────
-- Campo específico de homicídio: o instrumento/meio empregado. Fica na
-- tabela única de ocorrências (NULL/'' para apreensão e prisão), seguindo
-- a modelagem MVP de app.incidents.
--
-- '' = não informado (default), o que mantém compatível todo registro já
-- existente. `means_detail` é texto livre para qualificar o meio quando
-- 'outros' não basta.
ALTER TABLE app.incidents
  ADD COLUMN means        text NOT NULL DEFAULT '',
  ADD COLUMN means_detail text NOT NULL DEFAULT '';

ALTER TABLE app.incidents
  ADD CONSTRAINT incidents_means_check
  CHECK (means IN ('', 'paf', 'arma_branca', 'asfixia', 'contundente', 'outros'));

-- Agregação por meio no mapa do crime (ex.: "% PAF no mês").
CREATE INDEX incidents_means_idx ON app.incidents (means) WHERE deleted_at IS NULL;

-- Índice de suporte à consulta geográfica do mapa: só pontos plotáveis.
CREATE INDEX incidents_geo_idx
  ON app.incidents (occurred_on DESC)
  WHERE deleted_at IS NULL AND latitude IS NOT NULL AND longitude IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.incidents_geo_idx;
DROP INDEX IF EXISTS app.incidents_means_idx;
ALTER TABLE app.incidents DROP CONSTRAINT IF EXISTS incidents_means_check;
ALTER TABLE app.incidents DROP COLUMN IF EXISTS means_detail;
ALTER TABLE app.incidents DROP COLUMN IF EXISTS means;
-- +goose StatementEnd

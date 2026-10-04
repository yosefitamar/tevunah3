-- +goose Up
-- +goose StatementBegin

-- ─── Coordenada da ocorrência do relatório operacional ────────────────
-- O relatório traz o endereço em texto; a coordenada sai do geocodificador
-- da agência (Nominatim próprio, GEOCODER_URL — nenhum endereço de ocorrência
-- vai a serviço de terceiros) ou do analista.
--
-- geo_precision diz o quanto confiar no ponto automático:
--   'porta'  — achou o número na rua;
--   'rua'    — achou a rua, não o número;
--   'bairro' — só o bairro foi localizado: ponto APROXIMADO (centro dele).
-- geo_source: 'auto' (geocodificador) ou 'manual' (o analista informou — e
-- aí a correção de endereço não refaz a coordenada).
--
-- Com o geocodificador ligado, ocorrência sem coordenada fica com a
-- pendência "coordenada" (internal/sipom) até ser localizada.
ALTER TABLE app.ops_occurrences
  ADD COLUMN latitude      double precision NULL,
  ADD COLUMN longitude     double precision NULL,
  ADD COLUMN geo_precision text NOT NULL DEFAULT ''
    CHECK (geo_precision IN ('', 'porta', 'rua', 'bairro')),
  ADD COLUMN geo_source    text NOT NULL DEFAULT ''
    CHECK (geo_source IN ('', 'auto', 'manual'));

-- Só pontos plotáveis, para a camada de produtividade do mapa.
CREATE INDEX ops_occurrences_geo_idx
  ON app.ops_occurrences (occurred_on DESC)
  WHERE deleted_at IS NULL AND latitude IS NOT NULL AND longitude IS NOT NULL;

-- Cache do geocodificador: o mesmo endereço não é consultado duas vezes (a
-- prévia da importação e a gravação leem o mesmo PDF; o batalhão volta aos
-- mesmos endereços). geo_precision vazio = endereço não localizado, guardado
-- para não insistir a cada leitura — vale por alguns dias (internal/geocode).
CREATE TABLE app.geocode_cache (
  query_key     text PRIMARY KEY,
  latitude      double precision NULL,
  longitude     double precision NULL,
  geo_precision text NOT NULL DEFAULT '',
  looked_up_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.geocode_cache;
DROP INDEX IF EXISTS app.ops_occurrences_geo_idx;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS geo_source,
  DROP COLUMN IF EXISTS geo_precision,
  DROP COLUMN IF EXISTS longitude,
  DROP COLUMN IF EXISTS latitude;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- ─── Coordenada pela sede do município ────────────────────────────────
-- Endereço de zona rural (sítio, estrada vicinal, localidade) e de loteamento
-- novo quase nunca está no mapa, e a ocorrência ficava sem ponto. Quando nem
-- a rua nem o bairro são achados, o geocodificador passa a devolver a sede do
-- município, como último recurso:
--   'cidade' — ponto APROXIMADO, a sede do município.
ALTER TABLE app.ops_occurrences
  DROP CONSTRAINT ops_occurrences_geo_precision_check,
  ADD CONSTRAINT ops_occurrences_geo_precision_check
    CHECK (geo_precision IN ('', 'porta', 'rua', 'bairro', 'cidade'));

-- Os "não localizado" guardados no cache valeriam por dias e esconderiam a
-- regra nova: saem para a próxima busca consultar de novo.
DELETE FROM app.geocode_cache WHERE geo_precision = '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE app.ops_occurrences
   SET latitude = NULL, longitude = NULL, geo_precision = '', geo_source = ''
 WHERE geo_precision = 'cidade';
DELETE FROM app.geocode_cache WHERE geo_precision = 'cidade';
ALTER TABLE app.ops_occurrences
  DROP CONSTRAINT ops_occurrences_geo_precision_check,
  ADD CONSTRAINT ops_occurrences_geo_precision_check
    CHECK (geo_precision IN ('', 'porta', 'rua', 'bairro'));
-- +goose StatementEnd

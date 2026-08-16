-- +goose Up
-- +goose StatementBegin

-- ─── Recorte territorial ──────────────────────────────────────────────
-- Município e bairro da ocorrência. Coordenadas plotam o ponto; município
-- e bairro permitem o recorte e a agregação estatística por território,
-- que é como a agência lê o mapa do crime.
--
-- Município vem de lista fechada na UI (municípios do Ceará) justamente
-- para não fragmentar a agregação em grafias divergentes; bairro é livre
-- com autocompletar sobre os valores já cadastrados. Ambos gravados em
-- MAIÚSCULAS pelo backend, como o resto dos campos textuais do app.
ALTER TABLE app.incidents
  ADD COLUMN city         text NOT NULL DEFAULT '',
  ADD COLUMN neighborhood text NOT NULL DEFAULT '';

CREATE INDEX incidents_city_idx
  ON app.incidents (city) WHERE deleted_at IS NULL;
CREATE INDEX incidents_city_neighborhood_idx
  ON app.incidents (city, neighborhood) WHERE deleted_at IS NULL;

-- ─── Participação da INTEL ────────────────────────────────────────────
-- Removida: o campo não se sustentou no uso — toda ocorrência tramita pela
-- agência, então o marcador não separava nada. Down recria a coluna (os
-- valores anteriores não são recuperáveis).
ALTER TABLE app.incidents DROP COLUMN intel_participation;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.incidents
  ADD COLUMN intel_participation boolean NOT NULL DEFAULT false;
DROP INDEX IF EXISTS app.incidents_city_neighborhood_idx;
DROP INDEX IF EXISTS app.incidents_city_idx;
ALTER TABLE app.incidents DROP COLUMN IF EXISTS neighborhood;
ALTER TABLE app.incidents DROP COLUMN IF EXISTS city;
-- +goose StatementEnd

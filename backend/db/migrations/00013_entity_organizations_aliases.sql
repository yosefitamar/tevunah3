-- +goose Up
-- +goose StatementBegin

-- Organizações têm aliases/siglas (ex.: "CV" para "Comando Vermelho").
-- Mesma modelagem usada em entity_persons.aliases (text[]). Por convenção, o
-- primeiro elemento é a sigla primária — usada como rótulo prioritário na UI.
ALTER TABLE app.entity_organizations
  ADD COLUMN aliases text[] NOT NULL DEFAULT '{}';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.entity_organizations
  DROP COLUMN aliases;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- O campo SEDE/HQ saiu do escopo do MVP de organizações (decisão de UX:
-- localização da sede vai depender da modelagem futura de relacionamento
-- entre organização e entidade lugar). Drop limpo para evitar speculative state.
ALTER TABLE app.entity_organizations DROP COLUMN hq;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.entity_organizations ADD COLUMN hq text NULL;
-- +goose StatementEnd

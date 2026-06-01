-- +goose Up
-- +goose StatementBegin

-- Exclusão de qualificação é hard delete (DELETE FROM app.report_qualifications),
-- mas o role do app (tevunah_app) só tinha SELECT/INSERT/UPDATE no schema app —
-- então o DELETE falhava com "permission denied" e a qualificação não saía.
-- Concede DELETE nas duas tabelas (licenças saem por ON DELETE CASCADE, mas o
-- grant explícito evita surpresa caso a cascata seja removida no futuro).
-- Mesmo padrão de exceção já usado em user_roles (00005) e entity_tags (00011).
GRANT DELETE ON app.report_qualifications  TO tevunah_app;
GRANT DELETE ON app.report_qualif_licencas TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.report_qualif_licencas FROM tevunah_app;
REVOKE DELETE ON app.report_qualifications  FROM tevunah_app;
-- +goose StatementEnd

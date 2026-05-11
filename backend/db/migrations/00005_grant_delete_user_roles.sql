-- +goose Up
-- +goose StatementBegin

-- Junction app.user_roles é a única tabela em que removemos linhas como parte
-- do fluxo normal (alterar conjunto de papéis de um agente = DELETE + INSERT).
-- A mudança fica auditada no audit.audit_log com before/after de roles[],
-- então a perda da linha aqui não destrói rastro forense.
--
-- Demais tabelas em app.* seguem sem DELETE para tevunah_app — soft-delete
-- via deleted_at/status é o caminho.
GRANT DELETE ON app.user_roles TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.user_roles FROM tevunah_app;
-- +goose StatementEnd

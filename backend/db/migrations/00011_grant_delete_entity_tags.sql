-- +goose Up
-- +goose StatementBegin

-- entity_tags é tabela de junção: o fluxo de Update da entidade substitui o
-- conjunto de tags (DELETE + INSERT). A mudança fica auditada no audit.audit_log
-- via before/after da entidade (tags listadas no payload), então a perda da
-- linha aqui não destrói rastro forense — mesma justificativa de user_roles.
GRANT DELETE ON app.entity_tags TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.entity_tags FROM tevunah_app;
-- +goose StatementEnd

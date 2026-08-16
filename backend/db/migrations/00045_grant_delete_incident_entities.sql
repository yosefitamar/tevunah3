-- +goose Up
-- +goose StatementBegin

-- incident_entities é tabela de junção: desvincular um envolvido da
-- ocorrência (DELETE /api/incidents/{id}/entities/{eid}) apaga a linha.
-- Sem este GRANT o endpoint falha com "permission denied" — o papel da
-- aplicação só recebe DELETE onde o fluxo realmente precisa.
--
-- A remoção fica auditada em audit.audit_log pela ação
-- incident.entity.remove (com o entity_id no before), então a perda da
-- linha não destrói rastro forense — mesma justificativa de entity_tags
-- e user_roles.
GRANT DELETE ON app.incident_entities TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.incident_entities FROM tevunah_app;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Soft delete pra rascunhos (status='criado'). Mantém a linha no banco com
-- deleted_at preenchido — a auditoria/histórico preserva referências, mas a
-- linha some das queries normais (todas filtram deleted_at IS NULL).
ALTER TABLE app.reports
  ADD COLUMN deleted_at timestamptz NULL,
  ADD COLUMN deleted_by uuid NULL REFERENCES app.users(id);

-- Index parcial — só linhas vivas precisam de busca rápida.
CREATE INDEX reports_alive_idx ON app.reports (deleted_at) WHERE deleted_at IS NULL;

INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('analista',      'report.destroy', true, false, NULL),
  ('gestor',        'report.destroy', true, false, NULL),
  ('administrador', 'report.destroy', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'report.destroy';
DROP INDEX IF EXISTS app.reports_alive_idx;
ALTER TABLE app.reports DROP COLUMN IF EXISTS deleted_by;
ALTER TABLE app.reports DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd

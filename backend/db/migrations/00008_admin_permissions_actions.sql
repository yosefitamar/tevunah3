-- +goose Up
-- +goose StatementBegin

-- Actions que governam a tela ADMIN. Por ora só o administrador acessa;
-- gestor pode ganhar acesso depois se a aba de Dispositivos for compartilhada.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  ('administrador', 'admin.permissions.read',   true, false, NULL),
  ('administrador', 'admin.permissions.update', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM app.permissions
 WHERE action IN ('admin.permissions.read', 'admin.permissions.update');

-- +goose StatementEnd

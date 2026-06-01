-- +goose Up
-- +goose StatementBegin

-- Papéis customizados: a partir daqui o conjunto de papéis é DADO, não código.
-- is_system marca os 4 papéis embutidos (não podem ser excluídos/renomeados de
-- forma destrutiva). Papéis criados pelo admin nascem com is_system=false.
ALTER TABLE app.roles
  ADD COLUMN is_system boolean NOT NULL DEFAULT false;

UPDATE app.roles
   SET is_system = true
 WHERE codename IN ('agente', 'analista', 'gestor', 'administrador');

-- Ações de gestão de papéis — só o administrador as recebe por padrão. Como a
-- matriz agora é totalmente editável, demais papéis podem ganhá-las pela UI.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'role.read',   true, false, NULL),
  ('administrador', 'role.create', true, false, NULL),
  ('administrador', 'role.update', true, false, NULL),
  ('administrador', 'role.delete', true, false, NULL);

-- Excluir um papel customizado remove suas linhas da matriz em transação, e o
-- próprio papel — o role do app (tevunah_app) precisa de DELETE nessas tabelas
-- (antes só SELECT/INSERT/UPDATE). DELETE em app.roles é restrito a papéis
-- não-sistema pela lógica do repo.
GRANT DELETE ON app.permissions TO tevunah_app;
GRANT DELETE ON app.roles TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.roles FROM tevunah_app;
REVOKE DELETE ON app.permissions FROM tevunah_app;
DELETE FROM app.permissions WHERE action IN ('role.read','role.create','role.update','role.delete');
ALTER TABLE app.roles DROP COLUMN is_system;
-- +goose StatementEnd

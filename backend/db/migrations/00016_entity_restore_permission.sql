-- +goose Up
-- +goose StatementBegin

-- Restauração de entidades soft-deletadas (Lixeira). Mesmo conjunto de papéis
-- com permissão de exclusão (gestor + administrador), por consistência: quem
-- pode mandar para a lixeira pode tirar de lá.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  ('gestor',        'entity.restore', true, false, NULL),
  ('administrador', 'entity.restore', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'entity.restore';
-- +goose StatementEnd

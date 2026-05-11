-- +goose Up
-- +goose StatementBegin

-- Decisão de matriz (2026-05-10): administrador pode desativar agentes
-- sem aprovação dupla. Demais ações sensíveis seguem exigindo 4-eyes.
UPDATE app.permissions
   SET requires_dual_approval = false,
       approver_role          = NULL,
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND action    = 'user.deactivate';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

UPDATE app.permissions
   SET requires_dual_approval = true,
       approver_role          = 'gestor',
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND action    = 'user.deactivate';

-- +goose StatementEnd

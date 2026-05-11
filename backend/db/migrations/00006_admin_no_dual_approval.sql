-- +goose Up
-- +goose StatementBegin

-- Decisão de matriz (2026-05-10): administrador não exige 4-eyes para nenhuma
-- de suas ações. O 4-eyes permanece disponível como mecanismo da matriz e pode
-- ser reativado por linha; aqui apenas desligamos para o papel administrador.
-- Demais papéis seguem como estavam.
UPDATE app.permissions
   SET requires_dual_approval = false,
       approver_role          = NULL,
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND requires_dual_approval = true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restaura 4-eyes para as ações sensíveis do administrador que historicamente
-- exigiam aprovação de gestor. Mantém user.deactivate sem 4-eyes (decisão 00004
-- continua válida).
UPDATE app.permissions
   SET requires_dual_approval = true,
       approver_role          = 'gestor',
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND action IN ('user.role.assign', 'user.clearance.set', 'user.totp.reset');

-- +goose StatementEnd

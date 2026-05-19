-- +goose Up
-- +goose StatementBegin

-- Reset de TOTP vira ação direta do admin (sem 4-eyes). O fluxo seguro
-- agora vive no enrollment: admin apaga o secret, agente recebe novo QR no
-- primeiro login. Admin nunca enxerga o secret. Aprovação dupla deixou de
-- ser necessária.
UPDATE app.permissions
   SET requires_dual_approval = false,
       approver_role          = NULL,
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND action    = 'user.totp.reset';

-- Edição de perfil de outros agentes (display_name + email). Direto, com
-- audit. Edição de email é credencial-relevante mas não habilita login
-- sozinha (sem senha).
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'user.update.others', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE role_code = 'administrador' AND action = 'user.update.others';

UPDATE app.permissions
   SET requires_dual_approval = true,
       approver_role          = 'gestor',
       updated_at             = now()
 WHERE role_code = 'administrador'
   AND action    = 'user.totp.reset';
-- +goose StatementEnd

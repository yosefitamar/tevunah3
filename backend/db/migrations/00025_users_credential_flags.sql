-- +goose Up
-- +goose StatementBegin

-- Flags pra forçar reconfiguração de credenciais no próximo login.
-- Cobrem os fluxos:
--   - must_change_password = true → admin resetou a senha (gerou temporária);
--     login normal funciona, mas a sessão indica que o agente precisa trocar
--     a senha antes de prosseguir.
--   - must_setup_totp     = true → admin resetou o TOTP. O secret é apagado;
--     o próximo login dispensa o código TOTP e leva o agente a uma tela de
--     enrollment onde escaneia o novo QR. Admin nunca vê o novo secret.
ALTER TABLE app.users
  ADD COLUMN must_change_password boolean NOT NULL DEFAULT false,
  ADD COLUMN must_setup_totp      boolean NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.users
  DROP COLUMN must_change_password,
  DROP COLUMN must_setup_totp;
-- +goose StatementEnd

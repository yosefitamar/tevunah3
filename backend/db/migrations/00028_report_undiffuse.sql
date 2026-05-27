-- +goose Up
-- +goose StatementBegin

-- Permissão para reverter a difusão de um RI (devolver a status='criado').
-- Restrita a administradores: implica re-abrir documento já com número
-- alocado pra edição, então tem que ficar fora do alcance de gestor/analista.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'report.undiffuse', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'report.undiffuse';
-- +goose StatementEnd

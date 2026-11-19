-- +goose Up
-- +goose StatementBegin

-- Print (captura de tela) do pedido que originou a consulta — a mensagem, o
-- e-mail ou o ofício em que o solicitante pediu a pesquisa. Opcional.
ALTER TABLE app.consultas
  ADD COLUMN print_path text NULL;   -- filename em PHOTO_DIR

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.consultas DROP COLUMN IF EXISTS print_path;
-- +goose StatementEnd

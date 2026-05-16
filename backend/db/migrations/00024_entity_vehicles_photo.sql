-- +goose Up
-- +goose StatementBegin

-- Adiciona foto primária para veículos (mesma estratégia de person/place):
-- filename é resolvido pelo handler de upload sob PHOTO_DIR. Galeria
-- adicional já é polimórfica em entity_photos.
ALTER TABLE app.entity_vehicles
  ADD COLUMN photo_path text NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.entity_vehicles DROP COLUMN photo_path;
-- +goose StatementEnd

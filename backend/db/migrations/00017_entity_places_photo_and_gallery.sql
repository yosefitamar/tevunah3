-- +goose Up
-- +goose StatementBegin

-- 1) photo_path em entity_places — espelha entity_persons.photo_path.
--    Mesma convenção: filename relativo (ex.: "<entity_uuid>.jpg") sob PHOTO_DIR.
ALTER TABLE app.entity_places
  ADD COLUMN photo_path text NULL;

-- 2) Galeria de fotos adicionais por entidade (qualquer kind).
--    photo_path é "<photo_uuid>.<ext>" sob PHOTO_DIR (não colide com a foto
--    primária, que usa "<entity_uuid>.<ext>"). caption é livre e pode ser
--    vazia. ord define ordem visual; sort estável por (ord, created_at).
--    Soft delete via deleted_at preserva o registro p/ auditoria; o arquivo
--    em disco é removido fisicamente no DELETE pelo handler.
CREATE TABLE app.entity_photos (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_id    uuid NOT NULL REFERENCES app.entities(id),
  photo_path   text NOT NULL,
  caption      text NOT NULL DEFAULT '',
  mime         text NOT NULL,
  ord          integer NOT NULL DEFAULT 0,
  created_at   timestamptz NOT NULL DEFAULT now(),
  created_by   uuid NOT NULL REFERENCES app.users(id),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  updated_by   uuid NOT NULL REFERENCES app.users(id),
  deleted_at   timestamptz NULL,
  deleted_by   uuid NULL REFERENCES app.users(id)
);

CREATE INDEX entity_photos_entity_idx
  ON app.entity_photos (entity_id, ord, created_at)
  WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_photos_entity_idx;
DROP TABLE IF EXISTS app.entity_photos;
ALTER TABLE app.entity_places DROP COLUMN photo_path;
-- +goose StatementEnd

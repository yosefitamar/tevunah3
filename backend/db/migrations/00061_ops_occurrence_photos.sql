-- +goose Up
-- +goose StatementBegin

-- ─── Fotos da ocorrência do relatório operacional ─────────────────────
-- O relatório do CPRAIO não traz imagem: a foto da apreensão, da prisão ou
-- do local é anexada pelo analista na ficha e vai ao SIPOM junto com a
-- ocorrência (aba Fotos), em base64 no corpo do envio.
--
-- O arquivo fica em PHOTO_DIR ("ops-<uuid>.<ext>"); aqui só o registro. A
-- exclusão é lógica (o papel da aplicação não tem DELETE no schema): o
-- arquivo sai do disco e a linha fica, com quem e quando removeu.
CREATE TABLE app.ops_occurrence_photos (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  photo_path     text NOT NULL,
  mime           text NOT NULL CHECK (mime IN ('image/jpeg', 'image/png')),
  size_bytes     integer NOT NULL CHECK (size_bytes > 0),
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     uuid NOT NULL REFERENCES app.users(id),
  deleted_at     timestamptz NULL,
  deleted_by     uuid NULL REFERENCES app.users(id)
);

CREATE INDEX ops_occurrence_photos_occ_idx
  ON app.ops_occurrence_photos (occurrence_id, created_at)
  WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.ops_occurrence_photos;
-- +goose StatementEnd

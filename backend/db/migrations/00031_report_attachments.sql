-- +goose Up
-- +goose StatementBegin

-- Anexos de relatório. Casos de uso primários:
--   1. Importação do legado: preservar o PDF original gerado pelo sistema
--      antigo (registro oficial fiel) como anexo do RI novo.
--   2. Futuro: upload manual de PDFs/imagens via UI.
--
-- O arquivo físico vive em PHOTO_DIR/report-attachments/<filename>. O nome
-- canônico é o próprio UUID do registro + extensão, e original_name guarda
-- como o arquivo chegou (pra exibir na UI) — análogo ao padrão de photo_path
-- usado em entity_persons.
CREATE TABLE app.report_attachments (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id     uuid NOT NULL REFERENCES app.reports(id) ON DELETE CASCADE,
  filename      text NOT NULL,            -- <uuid>.<ext> em PHOTO_DIR/report-attachments
  original_name text NOT NULL,            -- nome exibido na UI ("RI-2025-001.pdf")
  mime          text NOT NULL,
  size_bytes    bigint NOT NULL,
  uploaded_at   timestamptz NOT NULL DEFAULT now(),
  uploaded_by   uuid NOT NULL REFERENCES app.users(id)
);

CREATE INDEX report_attachments_report_idx
  ON app.report_attachments (report_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.report_attachments;
-- +goose StatementEnd

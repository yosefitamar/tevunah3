-- +goose Up
-- +goose StatementBegin

-- ─── Confidencialidade do relatório ─────────────────────────────────────
-- Nível de sigilo formal — substitui o "SECRETO" hardcoded no PDF.
-- Default 'secreto' mantém o comportamento atual para RIs existentes.
ALTER TABLE app.reports
  ADD COLUMN confidentiality text NOT NULL DEFAULT 'secreto'
    CHECK (confidentiality IN ('sigiloso', 'secreto', 'ultrassecreto'));

-- ─── Visibilidade ───────────────────────────────────────────────────────
-- 'aberto' (default): visível pra todo mundo com permissão report.read.
-- 'restrito':         visível apenas pro autor + admins + report_viewers.
ALTER TABLE app.reports
  ADD COLUMN visibility text NOT NULL DEFAULT 'aberto'
    CHECK (visibility IN ('aberto', 'restrito'));

-- ─── report_viewers ─────────────────────────────────────────────────────
-- Lista de compartilhamento explícito. Autor + admin sempre veem o
-- relatório, independente desta tabela; aqui ficam os adicionais que o
-- autor liberou quando visibility='restrito'.
CREATE TABLE app.report_viewers (
  report_id   uuid NOT NULL REFERENCES app.reports(id) ON DELETE CASCADE,
  user_id     uuid NOT NULL REFERENCES app.users(id),
  granted_by  uuid NOT NULL REFERENCES app.users(id),
  granted_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (report_id, user_id)
);
CREATE INDEX report_viewers_user_idx ON app.report_viewers (user_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.report_viewers;
ALTER TABLE app.reports DROP COLUMN IF EXISTS visibility;
ALTER TABLE app.reports DROP COLUMN IF EXISTS confidentiality;
-- +goose StatementEnd

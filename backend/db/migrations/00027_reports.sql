-- +goose Up
-- +goose StatementBegin

-- ─── reports ──────────────────────────────────────────────────────────
-- Documento de inteligência. Status segue máquina:
--   criado → difundido → arquivado
-- Edição só é permitida em "criado". A numeração (NN/AAAA) é alocada na
-- transição criado→difundido, via sequência por ano. Rascunhos descartados
-- não consomem número.
CREATE TABLE app.reports (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind              text NOT NULL DEFAULT 'interno'
                    CHECK (kind IN ('interno')),
  status            text NOT NULL DEFAULT 'criado'
                    CHECK (status IN ('criado', 'difundido', 'arquivado')),

  -- Numeração: NULL enquanto em rascunho; preenchido na difusão.
  -- Unique parcial (year, seq) garante NN/AAAA único por ano.
  seq               int  NULL,
  year              int  NULL,

  -- Cabeçalho do relatório (campos do bloco superior do template).
  doc_date          date NOT NULL DEFAULT CURRENT_DATE,
  subject           text NOT NULL DEFAULT '',
  origin            text NOT NULL DEFAULT '',
  diffusion         text NOT NULL DEFAULT '',
  prior_diffusion   text NOT NULL DEFAULT '',
  reference         text NOT NULL DEFAULT '',
  attachments       text NOT NULL DEFAULT '',

  -- Corpo (HTML produzido pelo editor rich text).
  body_html         text NOT NULL DEFAULT '',

  -- Auditoria/transições.
  created_at        timestamptz NOT NULL DEFAULT now(),
  created_by        uuid NOT NULL REFERENCES app.users(id),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  updated_by        uuid NULL REFERENCES app.users(id),
  diffused_at       timestamptz NULL,
  diffused_by       uuid NULL REFERENCES app.users(id),
  archived_at       timestamptz NULL,
  archived_by       uuid NULL REFERENCES app.users(id)
);

-- Unique de (year, seq) só onde seq IS NOT NULL (rascunhos compartilham NULL).
CREATE UNIQUE INDEX reports_number_uniq
  ON app.reports (year, seq)
  WHERE seq IS NOT NULL;

CREATE INDEX reports_status_idx ON app.reports (status, doc_date DESC);
CREATE INDEX reports_created_by_idx ON app.reports (created_by);

-- ─── report_qualifications ───────────────────────────────────────────
-- Bloco repetível por relatório. Tipo CIVIL vincula obrigatoriamente uma
-- Entity (kind=person). MILITAR fica avulso (preenchimento manual).
-- "data" guarda o snapshot dos campos no momento da edição/difusão.
CREATE TABLE app.report_qualifications (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id     uuid NOT NULL REFERENCES app.reports(id) ON DELETE CASCADE,
  ord           int  NOT NULL DEFAULT 0,
  kind          text NOT NULL
                CHECK (kind IN ('militar', 'civil')),
  entity_id     uuid NULL REFERENCES app.entities(id),
  data          jsonb NOT NULL DEFAULT '{}'::jsonb,
  source        text NOT NULL DEFAULT '',
  consulted_at  timestamptz NULL,

  created_at    timestamptz NOT NULL DEFAULT now()
);
-- Regra de domínio: civil precisa de entity_id; militar não pode ter.
ALTER TABLE app.report_qualifications
  ADD CONSTRAINT report_qualifications_kind_entity_chk
  CHECK (
    (kind = 'civil'   AND entity_id IS NOT NULL) OR
    (kind = 'militar' AND entity_id IS NULL)
  );

CREATE INDEX report_qualifications_report_idx
  ON app.report_qualifications (report_id, ord);

-- ─── report_qualif_licencas ──────────────────────────────────────────
-- Tabela de licenças para qualificações militares (10 linhas no template).
CREATE TABLE app.report_qualif_licencas (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  qualification_id uuid NOT NULL
                  REFERENCES app.report_qualifications(id) ON DELETE CASCADE,
  ord             int  NOT NULL DEFAULT 0,
  boletim         text NOT NULL DEFAULT '',
  unidade         text NOT NULL DEFAULT '',
  publicacao      date NULL,
  data_inicio     date NULL,
  data_fim        date NULL,
  dias            int  NULL,
  cid             text NOT NULL DEFAULT ''
);
CREATE INDEX report_qualif_licencas_qualif_idx
  ON app.report_qualif_licencas (qualification_id, ord);

-- ─── report_downloads ────────────────────────────────────────────────
-- Forense: cada download do PDF cria uma linha aqui. Casa com os marcadores
-- invisíveis embebidos no PDF (DownloadID = id desta linha).
CREATE TABLE app.report_downloads (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  report_id           uuid NOT NULL REFERENCES app.reports(id),
  user_id             uuid NOT NULL REFERENCES app.users(id),
  session_token_hash  text NOT NULL,           -- sha256 do token de sessão (atribuição)
  ip                  text NOT NULL DEFAULT '',
  user_agent          text NOT NULL DEFAULT '',
  pdf_sha256          text NOT NULL DEFAULT '',
  downloaded_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX report_downloads_report_idx ON app.report_downloads (report_id, downloaded_at DESC);
CREATE INDEX report_downloads_user_idx   ON app.report_downloads (user_id, downloaded_at DESC);

-- ─── Permissões ──────────────────────────────────────────────────────
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('agente',        'report.read',     true, false, NULL),
  ('analista',      'report.read',     true, false, NULL),
  ('gestor',        'report.read',     true, false, NULL),
  ('administrador', 'report.read',     true, false, NULL),

  ('analista',      'report.create',   true, false, NULL),
  ('gestor',        'report.create',   true, false, NULL),
  ('administrador', 'report.create',   true, false, NULL),

  ('analista',      'report.update',   true, false, NULL),
  ('gestor',        'report.update',   true, false, NULL),
  ('administrador', 'report.update',   true, false, NULL),

  ('gestor',        'report.diffuse',  true, false, NULL),
  ('administrador', 'report.diffuse',  true, false, NULL),

  ('gestor',        'report.archive',  true, false, NULL),
  ('administrador', 'report.archive',  true, false, NULL),

  ('analista',      'report.download', true, false, NULL),
  ('gestor',        'report.download', true, false, NULL),
  ('administrador', 'report.download', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN (
   'report.read','report.create','report.update',
   'report.diffuse','report.archive','report.download'
 );
DROP TABLE IF EXISTS app.report_downloads;
DROP TABLE IF EXISTS app.report_qualif_licencas;
DROP TABLE IF EXISTS app.report_qualifications;
DROP TABLE IF EXISTS app.reports;
-- +goose StatementEnd

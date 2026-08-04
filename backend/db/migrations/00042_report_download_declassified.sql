-- +goose Up
-- +goose StatementBegin

-- Versão DESCARACTERIZADA do RI: mesmo conteúdo, sem identificação da
-- instituição nem do agente que gerou. É o formato que sai da instituição,
-- então ganha permissão própria em vez de herdar `report.download` — quem
-- baixa o RI normal não passa a poder produzir a versão sem procedência.
-- Analista fica de fora de propósito; gestor e administrador recebem.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('gestor',        'report.download_declassified', true, false, NULL),
  ('administrador', 'report.download_declassified', true, false, NULL);

-- Distingue no registro forense qual variante gerou aquele sha256. Sem isso
-- um hash de PDF descaracterizado ficaria indistinguível de um RI normal na
-- app.report_downloads, e é justamente o arquivo sem procedência que mais
-- importa rastrear se vazar.
ALTER TABLE app.report_downloads
  ADD COLUMN declassified boolean NOT NULL DEFAULT false;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.report_downloads DROP COLUMN IF EXISTS declassified;
DELETE FROM app.permissions WHERE action = 'report.download_declassified';
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Histórico criminal da pessoa como campo próprio, fora da descrição (que
-- fica para o texto curto do analista). Vem dos PDFs de consulta importados
-- no cadastro — RELATÓRIO DE CPF (peças judiciais: mandado, guia, alvará) e
-- CONSULTA INTEGRADA (ocorrências policiais como infrator/testemunha) — e é
-- editado na tela como lista (repeater).
--
-- Lista jsonb e não tabela 1-N: os registros são sempre lidos e gravados
-- juntos com a pessoa (o formulário envia a lista inteira) e entram na
-- auditoria no antes/depois da entidade. Formato de cada item validado no
-- backend (entities.normalizeCriminalRecords):
--   { category: "judicial"|"policial", role, title, detail, procedure_type,
--     number, piece, date, filed_on, agency, court, district, reason, source }
--
-- sip_number: "Nº SIP" da CONSULTA INTEGRADA (identificador da pessoa no
-- sistema policial de origem).
ALTER TABLE app.entity_persons
  ADD COLUMN sip_number       text  NULL,
  ADD COLUMN criminal_records jsonb NOT NULL DEFAULT '[]'::jsonb
    CONSTRAINT entity_persons_criminal_records_array
    CHECK (jsonb_typeof(criminal_records) = 'array');

CREATE INDEX entity_persons_sip_idx
  ON app.entity_persons (sip_number) WHERE sip_number IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_persons_sip_idx;
ALTER TABLE app.entity_persons
  DROP COLUMN criminal_records,
  DROP COLUMN sip_number;
-- +goose StatementEnd

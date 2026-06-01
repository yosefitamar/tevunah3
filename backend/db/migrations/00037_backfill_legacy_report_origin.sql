-- +goose Up
-- +goose StatementBegin

-- Relatórios importados do legado ficaram com origin em branco: no sistema
-- antigo a "Origem" era texto fixo no template, não uma coluna — então o
-- importador não tinha de onde mapear. Backfill: carimba o document_title
-- configurado (mesmo default dos RIs novos) nos RIs importados que estão com
-- origin vazio. Escopo restrito aos importados (import.import_map) pra não
-- tocar em RIs criados no sistema novo. No-op em ambientes sem import.
UPDATE app.reports r
   SET origin = s.document_title,
       updated_at = now()
  FROM app.system_settings s
 WHERE s.key = 'singleton'
   AND COALESCE(s.document_title, '') <> ''
   AND r.origin = ''
   AND r.id IN (
        SELECT new_uuid FROM import.import_map
         WHERE legacy_table = 'internal_reports'
   );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reversão best-effort: zera a origem apenas dos RIs importados.
UPDATE app.reports
   SET origin = ''
 WHERE id IN (
        SELECT new_uuid FROM import.import_map
         WHERE legacy_table = 'internal_reports'
   );
-- +goose StatementEnd

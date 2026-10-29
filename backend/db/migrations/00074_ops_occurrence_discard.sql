-- +goose Up
-- +goose StatementBegin

-- ─── Descarte da ocorrência importada ─────────────────────────────────
-- Ocorrência que não deveria estar no acervo (fora do batalhão, duplicada
-- com outra ficha, sem interesse para o SIPOM) é descartada pelo analista:
-- exclusão lógica (deleted_at/deleted_by), com o motivo. Ela sai das
-- listagens, da fila do SIPOM e dos dossiês, e a reimportação de um
-- relatório com a mesma ficha CIOPS não a traz de volta.
ALTER TABLE app.ops_occurrences
  ADD COLUMN deleted_reason text NOT NULL DEFAULT '';

CREATE INDEX ops_occurrences_discarded_ciops_idx ON app.ops_occurrences (ciops_record)
  WHERE deleted_at IS NOT NULL AND ciops_record <> '';

INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('analista',      'opsreport.delete', true, false, NULL),
  ('gestor',        'opsreport.delete', true, false, NULL),
  ('administrador', 'opsreport.delete', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'opsreport.delete';
DROP INDEX IF EXISTS app.ops_occurrences_discarded_ciops_idx;
ALTER TABLE app.ops_occurrences DROP COLUMN IF EXISTS deleted_reason;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- ─── Participação da inteligência ─────────────────────────────────────
-- Marca as ocorrências em que a SAI atuou (levantamento, informe, apoio).
-- No relatório operacional a marcação nasce automática: o histórico é
-- comparado com os termos de app.intel_keywords na importação. O analista
-- pode corrigir na ficha, e a correção manual prevalece sobre a regra.

-- Termos configuráveis (só o administrador mexe). A comparação ignora caixa,
-- acento e pontuação e exige palavra inteira — ver internal/intel.
CREATE TABLE app.intel_keywords (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  term        text NOT NULL,
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  created_by  uuid NULL REFERENCES app.users(id)
);
CREATE UNIQUE INDEX intel_keywords_term_uniq ON app.intel_keywords (app.norm_txt(term));

-- "SAI do 2º BPRAIO" é a expressão padronizada para os escreventes; as
-- demais cobrem as variações que ainda aparecem nas fichas.
INSERT INTO app.intel_keywords (term) VALUES
  ('SAI do 2º BPRAIO'),
  ('SAI 2º BPRAIO'),
  ('S.A.I.'),
  ('serviço de inteligência'),
  ('agência de inteligência'),
  ('P2');

-- intel_mode: 'auto' = decidido pelos termos; 'manual' = o analista marcou ou
-- desmarcou, e a reaplicação dos termos não toca mais na linha.
-- intel_matched: os termos que dispararam a marcação automática.
ALTER TABLE app.ops_occurrences
  ADD COLUMN intel_participation boolean NOT NULL DEFAULT false,
  ADD COLUMN intel_mode          text    NOT NULL DEFAULT 'auto'
    CHECK (intel_mode IN ('auto', 'manual')),
  ADD COLUMN intel_matched       text[]  NOT NULL DEFAULT '{}';

CREATE INDEX ops_occurrences_intel_idx
  ON app.ops_occurrences (occurred_on DESC)
  WHERE intel_participation AND deleted_at IS NULL;

-- Ocorrências (CVLI): cadastro curado, a marcação é do analista.
ALTER TABLE app.incidents
  ADD COLUMN intel_participation boolean NOT NULL DEFAULT false;

INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'intel.keywords.manage', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'intel.keywords.manage';
ALTER TABLE app.incidents DROP COLUMN IF EXISTS intel_participation;
DROP INDEX IF EXISTS app.ops_occurrences_intel_idx;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS intel_matched,
  DROP COLUMN IF EXISTS intel_mode,
  DROP COLUMN IF EXISTS intel_participation;
DROP TABLE IF EXISTS app.intel_keywords;
-- +goose StatementEnd

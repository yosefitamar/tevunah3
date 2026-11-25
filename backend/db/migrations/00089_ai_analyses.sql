-- +goose Up
-- +goose StatementBegin

-- Análises da IA guardadas por ocorrência: a tela mostra a última sem pedir
-- de novo (custo e espera), e uma nova verificação recebe a anterior — o que
-- o analista aplicou, não aceitou ou falhou ao gravar — para sair melhor.
CREATE TABLE app.ai_analyses (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  feature        text NOT NULL,                   -- ex.: "sipom_pendencias"
  occurrence_id  uuid NOT NULL REFERENCES app.ops_occurrences(id),
  previous_id    uuid REFERENCES app.ai_analyses(id),
  created_by     uuid NOT NULL REFERENCES app.users(id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  model          text NOT NULL DEFAULT '',
  input_tokens   integer NOT NULL DEFAULT 0,
  output_tokens  integer NOT NULL DEFAULT 0,
  cost_usd       numeric(10, 6) NOT NULL DEFAULT 0,
  sections       jsonb NOT NULL DEFAULT '[]',
  suggestions    jsonb NOT NULL DEFAULT '[]',
  -- Estado da ficha quando a análise vale (pendências + última correção):
  -- mudou depois, a tela avisa que a análise pode estar velha.
  fingerprint    text NOT NULL DEFAULT '',
  -- Resultado da revisão do analista, pela posição em suggestions.
  applied        jsonb NOT NULL DEFAULT '[]',     -- [0, 2]
  rejected       jsonb NOT NULL DEFAULT '[]',
  failed         jsonb NOT NULL DEFAULT '{}',     -- {"3": "mensagem do erro"}
  result_at      timestamptz
);

CREATE INDEX ai_analyses_occurrence_idx ON app.ai_analyses (feature, occurrence_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.ai_analyses;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- O assistente de IA mostra o que FOI GASTO (mês e ano), não estimativa: sai
-- o perfil de uso da estimativa e entra o registro de cada chamada à API.
ALTER TABLE app.user_ai_settings DROP COLUMN usage_profile;

-- Uma linha por chamada à API Anthropic feita por uma feature de IA, com os
-- tokens que a API devolveu em usage e o custo calculado na hora pelo preço
-- de tabela do modelo (o preço pode mudar depois; o custo gravado não muda).
-- Append-only: sem UPDATE pela aplicação, e o schema já nega DELETE.
CREATE TABLE app.ai_usage (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id            uuid NOT NULL REFERENCES app.users(id),
  model              text NOT NULL,
  feature            text NOT NULL,          -- qual feature chamou (ex.: "entidades.resumo")
  input_tokens       integer NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
  output_tokens      integer NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
  cache_write_tokens integer NOT NULL DEFAULT 0 CHECK (cache_write_tokens >= 0),
  cache_read_tokens  integer NOT NULL DEFAULT 0 CHECK (cache_read_tokens >= 0),
  cost_usd           numeric(14, 6) NOT NULL CHECK (cost_usd >= 0),
  created_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ai_usage_user_created_idx ON app.ai_usage (user_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.ai_usage;
ALTER TABLE app.user_ai_settings
  ADD COLUMN usage_profile jsonb NOT NULL DEFAULT '{}'::jsonb
    CHECK (jsonb_typeof(usage_profile) = 'object');
-- +goose StatementEnd

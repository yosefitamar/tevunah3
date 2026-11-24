-- +goose Up
-- +goose StatementBegin

-- Assistente de IA por usuário. Cada usuário cadastra a PRÓPRIA chave da API
-- Anthropic e escolhe o modelo; as features de IA usam a chave de quem está
-- logado (o custo cai na conta dele). Uma linha por usuário.
--
-- A chave nunca é guardada em claro: api_key_enc = nonce || ciphertext
-- (AES-256-GCM, chave mestra APP_SECRETS_KEY no ambiente do servidor, com o
-- user_id como dado associado — a cifra não abre se for copiada para outra
-- linha). api_key_hint guarda só o começo e o fim, para a tela identificar a
-- chave cadastrada.
--
-- "Remover a chave" anula as colunas (o schema não concede DELETE).
CREATE TABLE app.user_ai_settings (
  user_id          uuid PRIMARY KEY REFERENCES app.users(id),
  provider         text NOT NULL DEFAULT 'anthropic' CHECK (provider IN ('anthropic')),
  api_key_enc      bytea NULL,
  api_key_hint     text  NULL,
  key_verified_at  timestamptz NULL,
  -- Modelos que a chave enxerga (GET /v1/models na verificação).
  available_models text[] NOT NULL DEFAULT '{}',
  model            text NULL,
  -- Perfil de uso informado para a estimativa de custo (requisições/dia,
  -- tokens médios, dias úteis, cotação). Só alimenta a estimativa.
  usage_profile    jsonb NOT NULL DEFAULT '{}'::jsonb
                   CHECK (jsonb_typeof(usage_profile) = 'object'),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  CHECK ((api_key_enc IS NULL) = (api_key_hint IS NULL))
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.user_ai_settings;
-- +goose StatementEnd

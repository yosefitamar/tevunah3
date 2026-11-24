-- +goose Up
-- +goose StatementBegin

-- Orientações que os analistas escrevem para a IA ("em Cascavel, Zona Rural
-- vai como bairro X"). Entram no pedido à IA da feature correspondente.
-- Ciclo: pendente (vale só para o autor) → aprovada pelo administrador (vale
-- para todos) | recusada (não vale para ninguém) | desativada (idem, pelo
-- autor ou pelo administrador). Nada é apagado: o histórico fica na tabela e
-- na auditoria.
CREATE TABLE app.ai_guidance (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  feature       text NOT NULL,                  -- ex.: "sipom_pendencias"
  -- Tema da pendência: natureza | bairro | sexo | apreensoes | geral.
  topic         text NOT NULL CHECK (topic IN ('natureza', 'bairro', 'sexo', 'apreensoes', 'geral')),
  -- Cidade do catálogo do SIPOM (sipom.cidade.id); NULL = todas as cidades.
  cidade_id     integer,
  body          text NOT NULL CHECK (char_length(btrim(body)) BETWEEN 5 AND 1000),
  status        text NOT NULL DEFAULT 'pendente'
                  CHECK (status IN ('pendente', 'aprovada', 'recusada', 'desativada')),
  -- Ocorrência em que o analista escreveu a orientação (referência do caso).
  source_occurrence_id uuid REFERENCES app.ops_occurrences(id),
  created_by    uuid NOT NULL REFERENCES app.users(id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  decided_by    uuid REFERENCES app.users(id),
  decided_at    timestamptz,
  decision_note text NOT NULL DEFAULT '',
  -- Quantas sugestões da IA citaram a orientação.
  uses          integer NOT NULL DEFAULT 0 CHECK (uses >= 0),
  last_used_at  timestamptz
);

CREATE INDEX ai_guidance_feature_status_idx ON app.ai_guidance (feature, status);
CREATE INDEX ai_guidance_created_by_idx ON app.ai_guidance (created_by);

-- Criar orientação é de quem corrige ocorrências (opsreport.update); aprovar,
-- só o administrador.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'ai.guidance.approve', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'ai.guidance.approve';
DROP TABLE IF EXISTS app.ai_guidance;
-- +goose StatementEnd

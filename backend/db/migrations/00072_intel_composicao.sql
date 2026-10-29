-- +goose Up
-- +goose StatementBegin

-- ─── Dados funcionais do agente ───────────────────────────────────────
-- Matrícula, posto/graduação (sipom.postos_graduacoes.id), numeral e nome
-- de guerra. sipom_composition: o agente entra na composição (tipo de
-- policiamento Inteligência) das ocorrências com participação da
-- inteligência enviadas ao SIPOM.
ALTER TABLE app.users
  ADD COLUMN registration      text    NOT NULL DEFAULT '',
  ADD COLUMN rank_id           integer,
  ADD COLUMN number            text    NOT NULL DEFAULT '',
  ADD COLUMN war_name          text    NOT NULL DEFAULT '',
  ADD COLUMN sipom_composition boolean NOT NULL DEFAULT false;

-- ─── Composição da inteligência na ocorrência ─────────────────────────
-- Sem intel_composition_manual, a composição é a regra: os agentes marcados
-- no cadastro. Quando o analista insere ou retira alguém (ou a ocorrência é
-- enviada), a lista fica gravada aqui e deixa de seguir o cadastro.
ALTER TABLE app.ops_occurrences
  ADD COLUMN intel_composition_manual boolean NOT NULL DEFAULT false;

CREATE TABLE app.ops_occurrence_intel_officers (
  occurrence_id uuid    NOT NULL REFERENCES app.ops_occurrences(id) ON DELETE CASCADE,
  user_id       uuid    NOT NULL REFERENCES app.users(id),
  position      integer NOT NULL,
  funcao_id     integer NOT NULL, -- sipom.policiamentos_funcoes.id (Gestor, Agente, Analista)
  PRIMARY KEY (occurrence_id, user_id)
);
-- A lista é regravada inteira a cada escolha do analista.
GRANT DELETE ON app.ops_occurrence_intel_officers TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.ops_occurrence_intel_officers;
ALTER TABLE app.ops_occurrences DROP COLUMN IF EXISTS intel_composition_manual;
ALTER TABLE app.users
  DROP COLUMN IF EXISTS registration,
  DROP COLUMN IF EXISTS rank_id,
  DROP COLUMN IF EXISTS number,
  DROP COLUMN IF EXISTS war_name,
  DROP COLUMN IF EXISTS sipom_composition;
-- +goose StatementEnd

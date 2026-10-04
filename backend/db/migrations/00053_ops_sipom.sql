-- +goose Up
-- +goose StatementBegin

-- ─── Ocorrência no formato do SIPOM ───────────────────────────────────
-- Campos que o envio ao SIPOM exige, já nos códigos do destino (schema
-- sipom). A importação resolve o que dá a partir do PDF; o que não resolve
-- vira pendência. O texto original do relatório continua intocado nas
-- colunas de sempre — estas são a tradução para o destino.

ALTER TABLE app.ops_occurrences
  -- Natureza do fato (uma só no SIPOM). Preenchida pelo de-para (etapa 3)
  -- ou pelo analista.
  ADD COLUMN sipom_natureza_id integer NULL REFERENCES sipom.natureza_fatos(id),
  -- Endereço: logradouro e número separados; cidade e bairro casados com o
  -- catálogo (no SIPOM vão como texto, mas o id decide a área).
  ADD COLUMN sipom_logradouro  text    NOT NULL DEFAULT '',
  ADD COLUMN sipom_numeral     text    NOT NULL DEFAULT '',
  ADD COLUMN sipom_cidade_id   integer NULL REFERENCES sipom.cidade(id),
  ADD COLUMN sipom_bairro_id   integer NULL REFERENCES sipom.bairro(id),
  -- Área da unidade militar (companhia territorial do local do fato) e OPM
  -- que atendeu — ambas companhias do SIPOM.
  ADD COLUMN sipom_area_id     integer NULL REFERENCES sipom.companhias(id),
  ADD COLUMN sipom_opm_id      integer NULL REFERENCES sipom.companhias(id),
  -- Campos que o analista ajustou à mão: o recálculo não os sobrescreve.
  ADD COLUMN sipom_manual      text[]  NOT NULL DEFAULT '{}',
  -- O que impede o envio (códigos de internal/sipom), regravado a cada
  -- resolução. Vazio = pronta para enviar.
  ADD COLUMN sipom_pendencias  text[]  NOT NULL DEFAULT '{}';

CREATE INDEX ops_occurrences_sipom_pending_idx
  ON app.ops_occurrences ((cardinality(sipom_pendencias) > 0))
  WHERE deleted_at IS NULL;

-- Composição: tipo de policiamento e função de cada policial, derivados da
-- equipe (VTRA = Motorizado, RAIO = Motopatrulhamento) e da ordem na lista.
ALTER TABLE app.ops_occurrence_officers
  ADD COLUMN sipom_policiamento_tipo_id integer NULL REFERENCES sipom.policiamentos_tipos(id),
  ADD COLUMN sipom_funcao_id            integer NULL REFERENCES sipom.policiamentos_funcoes(id),
  -- Equipe do policial ("RAIO 01"). Com uma equipe só, vem da ficha; com
  -- várias, o analista define.
  ADD COLUMN sipom_equipe               text    NOT NULL DEFAULT '';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.ops_occurrences_sipom_pending_idx;
ALTER TABLE app.ops_occurrence_officers
  DROP COLUMN IF EXISTS sipom_equipe,
  DROP COLUMN IF EXISTS sipom_funcao_id,
  DROP COLUMN IF EXISTS sipom_policiamento_tipo_id;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS sipom_pendencias,
  DROP COLUMN IF EXISTS sipom_manual,
  DROP COLUMN IF EXISTS sipom_opm_id,
  DROP COLUMN IF EXISTS sipom_area_id,
  DROP COLUMN IF EXISTS sipom_bairro_id,
  DROP COLUMN IF EXISTS sipom_cidade_id,
  DROP COLUMN IF EXISTS sipom_numeral,
  DROP COLUMN IF EXISTS sipom_logradouro,
  DROP COLUMN IF EXISTS sipom_natureza_id;
-- +goose StatementEnd

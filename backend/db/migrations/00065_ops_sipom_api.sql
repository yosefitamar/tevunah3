-- +goose Up
-- +goose StatementBegin

-- ─── Envio pela API do SIPOM ──────────────────────────────────────────
-- O SIPOM entregou a API de integração (2026-10-19): POST assinado que cria
-- ou atualiza a ocorrência por origem.id. Aqui fica o resultado do último
-- envio. sipom_sent_at/by (00064) passam a ser gravados pelo envio que deu
-- certo; sem a API configurada, o analista ainda marca à mão.
ALTER TABLE app.ops_occurrences
  ADD COLUMN sipom_protocolo     uuid NULL,
  ADD COLUMN sipom_identificador text NOT NULL DEFAULT '',
  ADD COLUMN sipom_situacao      text NOT NULL DEFAULT '',
  ADD COLUMN sipom_avisos        jsonb NOT NULL DEFAULT '[]'::jsonb,
  -- Hash do corpo aceito por último: diz se a ficha mudou desde o envio.
  ADD COLUMN sipom_body_sha256   text NOT NULL DEFAULT '',
  -- Erro da última tentativa que falhou ('' = a última deu certo).
  ADD COLUMN sipom_erro          text NOT NULL DEFAULT '',
  ADD COLUMN sipom_tentativa_em  timestamptz NULL,
  -- O SIPOM respondeu 409: a ocorrência foi excluída lá e não é recriada.
  ADD COLUMN sipom_excluida_em   timestamptz NULL;

-- Sexo do envolvido para o SIPOM, quando o dossiê não diz (ou não há
-- dossiê): o SIPOM exige sexo de toda pessoa identificada.
ALTER TABLE app.ops_occurrence_people
  ADD COLUMN sipom_sexo smallint NULL CHECK (sipom_sexo IN (1, 2));

-- Ano de fabricação (obrigatório para placa nova no SIPOM) e do modelo do
-- veículo apreendido: o relatório não traz, o analista informa.
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN sipom_ano_fabricacao smallint NULL CHECK (sipom_ano_fabricacao BETWEEN 1900 AND 2100),
  ADD COLUMN sipom_ano_modelo     smallint NULL CHECK (sipom_ano_modelo BETWEEN 1900 AND 2100);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_vehicles
  DROP COLUMN IF EXISTS sipom_ano_modelo,
  DROP COLUMN IF EXISTS sipom_ano_fabricacao;
ALTER TABLE app.ops_occurrence_people
  DROP COLUMN IF EXISTS sipom_sexo;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS sipom_excluida_em,
  DROP COLUMN IF EXISTS sipom_tentativa_em,
  DROP COLUMN IF EXISTS sipom_erro,
  DROP COLUMN IF EXISTS sipom_body_sha256,
  DROP COLUMN IF EXISTS sipom_avisos,
  DROP COLUMN IF EXISTS sipom_situacao,
  DROP COLUMN IF EXISTS sipom_identificador,
  DROP COLUMN IF EXISTS sipom_protocolo;
-- +goose StatementEnd

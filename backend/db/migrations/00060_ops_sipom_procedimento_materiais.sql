-- +goose Up
-- +goose StatementBegin

-- ─── Procedimento e materiais nos códigos do SIPOM ────────────────────
-- Fase 2 do envio (docs/sipom-materiais.md): o procedimento (tipo, delegacia,
-- delegado) e os materiais apreendidos (armas, drogas, veículos) traduzidos
-- para as tabelas do SIPOM. O texto do relatório continua intocado nas
-- colunas de sempre; estas são a tradução. O que o analista fixar à mão não
-- é sobrescrito pelo recálculo (sipom_manual).

ALTER TABLE app.ops_occurrences
  -- Tipo do procedimento (IP, TCO, BO, Ato Infracional), delegacia e
  -- delegado. Fixados pelo analista entram em sipom_manual como
  -- 'procedimento', 'delegacia' e 'delegado'.
  ADD COLUMN sipom_procedimento_id integer NULL REFERENCES sipom.procedimentos(id),
  ADD COLUMN sipom_delegacia_id    integer NULL REFERENCES sipom.delegacias(id),
  ADD COLUMN sipom_delegado_id     integer NULL REFERENCES sipom.delegados(id);

ALTER TABLE app.ops_occurrence_weapons
  ADD COLUMN sipom_tipo_id    integer NULL REFERENCES sipom.arma_tipos(id),
  ADD COLUMN sipom_marca_id   integer NULL REFERENCES sipom.arma_marcas(id),
  ADD COLUMN sipom_calibre_id integer NULL REFERENCES sipom.arma_calibres(id),
  ADD COLUMN sipom_manual     boolean NOT NULL DEFAULT false;

ALTER TABLE app.ops_occurrence_drugs
  ADD COLUMN sipom_droga_id   integer NULL REFERENCES sipom.drogas(id),
  -- Quantidade na unidade da droga no SIPOM (gramas, comprimidos, ml…).
  ADD COLUMN sipom_quantidade numeric(14,3) NULL,
  ADD COLUMN sipom_manual     boolean NOT NULL DEFAULT false;

ALTER TABLE app.ops_occurrence_vehicles
  -- Códigos DENATRAN, como os selects do SIPOM enviam.
  ADD COLUMN sipom_tipo_codigo         integer NULL,
  ADD COLUMN sipom_cor_codigo          integer NULL,
  ADD COLUMN sipom_marca_modelo_codigo integer NULL,
  -- 1 = apreendido, 2 = recuperado; 0 = não decidido.
  ADD COLUMN sipom_situacao            smallint NOT NULL DEFAULT 0
    CHECK (sipom_situacao IN (0, 1, 2)),
  ADD COLUMN sipom_manual              boolean NOT NULL DEFAULT false;

-- ─── Termos aprendidos ────────────────────────────────────────────────
-- O relatório escreve em texto livre o que o SIPOM quer em lista: "DMC" é
-- a delegacia 201, "REVOLVER" é o tipo 2, "PRETO" é a cor 11. Cada escolha
-- do analista vira referência aqui (campo + termo → id no SIPOM) e vale para
-- as próximas ocorrências, como a referência de área. term_key é a forma de
-- comparação (internal/sipom); term guarda a grafia do relatório.
CREATE TABLE app.sipom_term_map (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  field      text NOT NULL,
  term       text NOT NULL,
  term_key   text NOT NULL,
  target_id  integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid NOT NULL REFERENCES app.users(id),
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid NOT NULL REFERENCES app.users(id),
  CONSTRAINT sipom_term_map_uniq UNIQUE (field, term_key)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.sipom_term_map;
ALTER TABLE app.ops_occurrence_vehicles
  DROP COLUMN IF EXISTS sipom_manual,
  DROP COLUMN IF EXISTS sipom_situacao,
  DROP COLUMN IF EXISTS sipom_marca_modelo_codigo,
  DROP COLUMN IF EXISTS sipom_cor_codigo,
  DROP COLUMN IF EXISTS sipom_tipo_codigo;
ALTER TABLE app.ops_occurrence_drugs
  DROP COLUMN IF EXISTS sipom_manual,
  DROP COLUMN IF EXISTS sipom_quantidade,
  DROP COLUMN IF EXISTS sipom_droga_id;
ALTER TABLE app.ops_occurrence_weapons
  DROP COLUMN IF EXISTS sipom_manual,
  DROP COLUMN IF EXISTS sipom_calibre_id,
  DROP COLUMN IF EXISTS sipom_marca_id,
  DROP COLUMN IF EXISTS sipom_tipo_id;
ALTER TABLE app.ops_occurrences
  DROP COLUMN IF EXISTS sipom_delegado_id,
  DROP COLUMN IF EXISTS sipom_delegacia_id,
  DROP COLUMN IF EXISTS sipom_procedimento_id;
-- +goose StatementEnd

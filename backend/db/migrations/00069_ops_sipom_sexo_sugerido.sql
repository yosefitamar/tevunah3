-- +goose Up
-- +goose StatementBegin

-- ─── Sexo sugerido do envolvido ──────────────────────────────────────
-- Quando nem o dossiê nem o analista dizem o sexo, o recálculo da tradução
-- sugere um: pelo primeiro nome (censo, app.nomes_sexo), pelo que os
-- analistas já marcaram para o nome (sipom_term_map, campo sexo_nome) ou,
-- para o único acusado da ficha, pelo histórico no feminino. A sugestão não
-- vai ao SIPOM: o analista confirma (sipom_sexo) antes.
ALTER TABLE app.ops_occurrence_people
  ADD COLUMN sipom_sexo_sugerido smallint NULL CHECK (sipom_sexo_sugerido IN (1, 2)),
  -- nome | aprendido | historico ('' = sem sugestão)
  ADD COLUMN sipom_sexo_fonte    text NOT NULL DEFAULT '';

-- ─── Linhas falsas de envolvido ──────────────────────────────────────
-- Relatórios impressos pelo navegador trazem no topo da página a data da
-- impressão ("09/01/26, 07:21  RELATÓRIO DE OCORRÊNCIAS") e no rodapé o
-- endereço do sistema com a página ("https://sgr.pm.ce.gov.br/... 15/33").
-- Quando a lista de envolvidos atravessava a quebra de página, as duas
-- linhas entravam como pessoas. O parser passou a ignorá-las; aqui saem as
-- que já foram gravadas.
DELETE FROM app.ops_occurrence_people
 WHERE entity_id IS NULL
   AND (name ~* '^https?://' OR name ~ '^\d{2}/\d{2}/\d{2}, \d{2}:\d{2}$');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrence_people
  DROP COLUMN IF EXISTS sipom_sexo_fonte,
  DROP COLUMN IF EXISTS sipom_sexo_sugerido;
-- +goose StatementEnd

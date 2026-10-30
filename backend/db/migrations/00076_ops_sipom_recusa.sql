-- +goose Up
-- +goose StatementBegin

-- ─── Recusa do SIPOM ──────────────────────────────────────────────────
-- O SIPOM recusou o último envio (4xx que não é limite de requisições): a
-- ocorrência sai das prontas, com a pendência envio_recusado, até um envio
-- dar certo. Falha passageira (SIPOM fora, sem resposta) não marca.
ALTER TABLE app.ops_occurrences
  ADD COLUMN sipom_recusada_em timestamptz NULL;

-- As recusas que já estão na fila (o recálculo acende a pendência).
UPDATE app.ops_occurrences
   SET sipom_recusada_em = sipom_tentativa_em
 WHERE deleted_at IS NULL AND sipom_sent_at IS NULL
   AND sipom_erro ~ '^SIPOM 4[0-9][0-9] ' AND sipom_erro !~ '^SIPOM 429 ';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ops_occurrences DROP COLUMN IF EXISTS sipom_recusada_em;
-- +goose StatementEnd

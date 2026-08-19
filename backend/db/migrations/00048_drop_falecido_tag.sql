-- +goose Up
-- +goose StatementBegin

-- ─── Aposenta a tag 'falecido' ────────────────────────────────────────
--
-- A tag era o registro de óbito do sistema antigo, preservado pela 00047
-- como rastro enquanto a conversão para a coluna `deceased` não estivesse
-- confirmada. Com o óbito virando atributo de primeira classe (selo na
-- listagem, data no dossiê, vínculo com a ocorrência), a tag passou a ser
-- uma segunda fonte de verdade que ninguém atualiza — e duas fontes que
-- discordam são pior que uma.
--
-- A remoção é condicionada a deceased = true: se alguma marcação tiver sido
-- desfeita à mão desde a 00047 (homônimo confirmado por engano), a tag fica
-- de pé e a informação não se perde silenciosamente.

DELETE FROM app.entity_tags t
 USING app.entity_persons p
 WHERE t.tag = 'falecido'
   AND p.entity_id = t.entity_id
   AND p.deceased;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Irreversível na prática: restaurar a tag exigiria saber quais óbitos
-- vieram do legado e quais nasceram no Tevunah, distinção que a coluna
-- `deceased` não guarda. A informação de óbito em si continua íntegra em
-- app.entity_persons.deceased.
SELECT 1;
-- +goose StatementEnd

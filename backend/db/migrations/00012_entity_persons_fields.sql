-- +goose Up
-- +goose StatementBegin

-- Reformulação dos campos de pessoa para o caso de uso real (inteligência
-- voltada a indivíduos no contexto brasileiro):
--   removidos: nationality, doc_type, doc_number, photo_url (substituído por
--              photo_path, que aponta para o arquivo em disco)
--   adicionados:
--     mother_name  — nome da mãe (campo investigativo essencial no BR)
--     cpf          — documento único nacional (texto, 11 dígitos sem máscara)
--     photo_path   — caminho relativo do arquivo (ex.: "<uuid>.jpg") sob o
--                    diretório configurado por PHOTO_DIR; servido por
--                    GET /api/entities/{id}/photo
--     orcrim_id    — referência a uma entidade do tipo organization marcada
--                    com a tag 'orcrim'. Validação semântica fica no app.
--
-- gender e date_of_birth permanecem como já estavam.

ALTER TABLE app.entity_persons
  DROP COLUMN nationality,
  DROP COLUMN doc_type,
  DROP COLUMN doc_number,
  DROP COLUMN photo_url,
  ADD COLUMN mother_name text NULL,
  ADD COLUMN cpf         text NULL,
  ADD COLUMN photo_path  text NULL,
  ADD COLUMN orcrim_id   uuid NULL REFERENCES app.entities(id);

CREATE INDEX entity_persons_cpf_idx
  ON app.entity_persons (cpf) WHERE cpf IS NOT NULL;
CREATE INDEX entity_persons_orcrim_idx
  ON app.entity_persons (orcrim_id) WHERE orcrim_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entity_persons_orcrim_idx;
DROP INDEX IF EXISTS app.entity_persons_cpf_idx;
ALTER TABLE app.entity_persons
  DROP COLUMN orcrim_id,
  DROP COLUMN photo_path,
  DROP COLUMN cpf,
  DROP COLUMN mother_name,
  ADD COLUMN nationality text NULL,
  ADD COLUMN doc_type    text NULL,
  ADD COLUMN doc_number  text NULL,
  ADD COLUMN photo_url   text NULL;
-- +goose StatementEnd

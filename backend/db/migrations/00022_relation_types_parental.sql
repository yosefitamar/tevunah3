-- +goose Up
-- +goose StatementBegin

-- Adiciona relações parentais explícitas ao catálogo de relation_type.
--
-- father_of / mother_of são direcionais (pai/mãe → filho) e substituem o uso
-- genérico de 'relative' quando o vínculo é sabidamente paternal/maternal.
-- O grafo se beneficia: o layout dagre top-down passa a colocar pais acima
-- dos filhos automaticamente, e o cadastro de pessoa pode auto-vincular a
-- mãe quando o usuário escolhe uma entidade existente no campo "mãe".

ALTER TABLE app.entity_links DROP CONSTRAINT entity_links_relation_type_check;
ALTER TABLE app.entity_links ADD CONSTRAINT entity_links_relation_type_check
  CHECK (relation_type IN (
    'associated_with',
    'owns',
    'spouse',
    'relative',
    'friend',
    'colleague',
    'partner',
    'member_of',
    'leader_of',
    'employee_of',
    'drives',
    'frequents',
    'subsidiary_of',
    'partnership',
    'based_at',
    'father_of',
    'mother_of'
  ));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE app.entity_links DROP CONSTRAINT entity_links_relation_type_check;
ALTER TABLE app.entity_links ADD CONSTRAINT entity_links_relation_type_check
  CHECK (relation_type IN (
    'associated_with',
    'owns',
    'spouse',
    'relative',
    'friend',
    'colleague',
    'partner',
    'member_of',
    'leader_of',
    'employee_of',
    'drives',
    'frequents',
    'subsidiary_of',
    'partnership',
    'based_at'
  ));

-- +goose StatementEnd

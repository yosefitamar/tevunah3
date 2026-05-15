-- +goose Up
-- +goose StatementBegin

-- Adiciona vínculos de irmandade ao catálogo. São simétricos pessoa↔pessoa:
--   sibling      = irmão(ã) — mesmo pai E mesma mãe.
--   half_sibling = meio-irmão(ã) — apenas pai OU apenas mãe em comum.
--
-- A inserção é automática: o domain dispara ResyncSiblings sempre que um
-- father_of ou mother_of é criado. half_sibling vira sibling automaticamente
-- quando o segundo parente coincide.

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
    'mother_of',
    'sibling',
    'half_sibling'
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
    'based_at',
    'father_of',
    'mother_of'
  ));
-- +goose StatementEnd

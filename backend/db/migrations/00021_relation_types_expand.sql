-- +goose Up
-- +goose StatementBegin

-- Expansão do catálogo de relation_type no app.entity_links.
--
-- Antes: apenas 'owns' e 'associated_with' (Phase 1 do módulo Veículos).
-- Agora: catálogo investigativo enxuto cobrindo pessoa↔pessoa (cônjuge, parente,
-- amigo, colega, sócio), pessoa↔organização (membro, líder, funcionário),
-- pessoa↔veículo (condutor), pessoa↔lugar (frequenta), organização↔organização
-- (parceria, subsidiária), organização↔lugar (sede).
--
-- A validação de PAR (from_kind × to_kind) por relation_type continua no
-- domain (entities.CreateLink) — a CHECK no banco só fecha o conjunto. Isso
-- mantém o vocabulário versionado em código sem precisar de mais um nível de
-- enforcement no SQL.

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

-- +goose Down
-- +goose StatementBegin

-- Cuidado: rollback rejeita linhas com relation_type fora do conjunto antigo.
-- Antes do rollback, soft-delete ou converta os registros para
-- 'associated_with'.
ALTER TABLE app.entity_links DROP CONSTRAINT entity_links_relation_type_check;
ALTER TABLE app.entity_links ADD CONSTRAINT entity_links_relation_type_check
  CHECK (relation_type IN ('owns', 'associated_with'));

-- +goose StatementEnd

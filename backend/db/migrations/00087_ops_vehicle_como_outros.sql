-- +goose Up
-- +goose StatementBegin

-- O analista (ou a IA, com a confirmação dele) decide mandar ao SIPOM como
-- material "Outros" um veículo que tem placa — por exemplo, placa nova que o
-- SIPOM recusou por falta do ano de fabricação, que ninguém sabe. Sem placa,
-- o veículo já vai como "Outros" (00084); esta é a escolha para os demais.
ALTER TABLE app.ops_occurrence_vehicles
  ADD COLUMN sipom_como_outros boolean NOT NULL DEFAULT false;

-- Tema "recusa" nas orientações para a IA: como tratar cada tipo de recusa
-- do SIPOM (ano de fabricação que falta, OPM não permitida…).
ALTER TABLE app.ai_guidance DROP CONSTRAINT ai_guidance_topic_check;
ALTER TABLE app.ai_guidance ADD CONSTRAINT ai_guidance_topic_check
  CHECK (topic IN ('natureza', 'bairro', 'sexo', 'apreensoes', 'recusa', 'geral'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.ai_guidance DROP CONSTRAINT ai_guidance_topic_check;
ALTER TABLE app.ai_guidance ADD CONSTRAINT ai_guidance_topic_check
  CHECK (topic IN ('natureza', 'bairro', 'sexo', 'apreensoes', 'geral'));
ALTER TABLE app.ops_occurrence_vehicles DROP COLUMN IF EXISTS sipom_como_outros;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Nível de ACESSO do relatório — comparado ao clearance_level do usuário para
-- decidir QUEM pode ver o RI. NÃO confundir com `confidentiality`
-- (sigiloso/secreto/ultrassecreto), que é a classificação LEGAL (LAI) ligada ao
-- PRAZO de restrição. São dimensões ortogonais.
--
-- Regra de leitura (não-admin, não-autor):
--   (visibility='aberto' OR é viewer) AND clearance_level >= required_clearance
-- Default 1: todo RI nasce visível a qualquer um com report.read (sem lockout);
-- o admin eleva o nível caso a caso.
ALTER TABLE app.reports
  ADD COLUMN required_clearance smallint NOT NULL DEFAULT 1
    CHECK (required_clearance BETWEEN 1 AND 5);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE app.reports DROP COLUMN required_clearance;
-- +goose StatementEnd

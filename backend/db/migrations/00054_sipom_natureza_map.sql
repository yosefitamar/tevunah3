-- +goose Up
-- +goose StatementBegin

-- ─── De-para de naturezas: relatório operacional → SIPOM ──────────────
-- O relatório do CPRAIO descreve a ação ("APRESENTAÇÃO E APREENSÃO DE
-- DROGAS") ou o crime com artigo ("FURTO - ART. 155/CPB"); o SIPOM aceita
-- uma natureza só, do catálogo dele. Esta tabela liga uma à outra.
--
-- source: a natureza como o relatório escreve, sem o artigo de lei — a
--   comparação ignora caixa, acento, pontuação e o trecho "- ART. ...".
-- confianca: 'direta' aplica sozinha; 'sugerida' aplica mas deixa a
--   ocorrência pendente até o analista confirmar.
-- prioridade: com várias naturezas na ficha, vale a de menor número (a
--   mais grave).
-- sipom_natureza_id NULL = sem correspondência (a ocorrência fica pendente
--   se não tiver outra natureza mapeada).
CREATE TABLE app.sipom_natureza_map (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source             text NOT NULL,
  sipom_natureza_id  integer NULL REFERENCES sipom.natureza_fatos(id),
  confianca          text NOT NULL DEFAULT 'sugerida' CHECK (confianca IN ('direta', 'sugerida')),
  prioridade         integer NOT NULL DEFAULT 100,
  updated_at         timestamptz NOT NULL DEFAULT now(),
  updated_by         uuid NULL REFERENCES app.users(id)
);
CREATE UNIQUE INDEX sipom_natureza_map_source_uniq ON app.sipom_natureza_map (app.norm_txt(source));

-- Carga inicial a partir das naturezas vistas nos relatórios (2026-10).
-- Tráfico e porte seguem o uso real no SIPOM (10 e 121), não as entradas
-- duplicadas mais novas do catálogo.
INSERT INTO app.sipom_natureza_map (source, sipom_natureza_id, confianca, prioridade) VALUES
  ('INTERVENÇÃO POLICIAL LETAL',                       160, 'direta',   10),
  ('TENTATIVA DE HOMICIDIO',                           NULL, 'sugerida', 15),
  ('ESTUPRO',                                          3,   'direta',   20),
  ('ORGANIZAÇÃO CRIMINOSA',                            183, 'direta',   30),
  ('TRÁFICO DE DROGAS',                                10,  'direta',   35),
  ('PORTE ILEGAL DE ARMA DE FOGO DE USO PERMITIDO',    121, 'direta',   40),
  ('POSSE ILEGAL DE ARMA DE FOGO DE USO PERMITIDO',    119, 'direta',   42),
  ('POSSE ILEGAL DE MUNIÇÃO DE USO PERMITIDO',         119, 'sugerida', 43),
  ('APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO',         76,  'sugerida', 45),
  ('APRESENTAÇÃO E APREENSÃO DE DROGAS',               10,  'sugerida', 46),
  ('CORRUPÇÃO DE MENORES',                             38,  'direta',   50),
  ('RECEPTAÇÃO',                                       24,  'direta',   55),
  ('ADULTERAÇÃO DE VEÍCULO',                           65,  'sugerida', 56),
  ('FURTO DE VEÍCULO',                                 8,   'direta',   57),
  ('FURTO',                                            6,   'sugerida', 58),
  ('EMBRIAGUEZ AO VOLANTE',                            82,  'sugerida', 60),
  ('DESOBEDIÊNCIA',                                    66,  'sugerida', 62),
  ('CONSUMO DE ENTORPECENTES',                         9,   'sugerida', 65),
  ('MANDADO DE PRISÃO',                                161, 'direta',   70),
  ('VEÍCULO LOCALIZADO',                               156, 'direta',   80),
  ('APRESENTAÇÃO E APREENSÃO DE OBJETOS',              NULL, 'sugerida', 90),
  ('OUTROS',                                           NULL, 'sugerida', 99);

-- Quem fixa a natureza à mão na ficha usa opsreport.update; o de-para em si
-- é do administrador.
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('administrador', 'sipom.mapping.manage', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action = 'sipom.mapping.manage';
DROP TABLE IF EXISTS app.sipom_natureza_map;
-- +goose StatementEnd

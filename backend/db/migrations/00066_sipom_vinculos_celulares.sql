-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_vinculos.py a partir do terceiro dump do
-- SIPOM (2026-10-19: vinculos e marca_celulares). Não editar à mão: regenere
-- pelo script.


-- ─── Vínculos de pessoa ──────────────────────────────────────────────
-- vinculo_id dos envolvidos na API do SIPOM. A ocorrência usa Vitima,
-- Infrator, Testemunha e os dois "não identificado"; os demais (parentesco,
-- advogado…) são vínculos entre pessoas no SIPOM.
CREATE TABLE sipom.vinculos (id integer PRIMARY KEY, nome text NOT NULL);

-- vinculos: 23 linhas
INSERT INTO sipom.vinculos (id, nome) VALUES
  (1, 'Conjuge'),
  (2, 'Filho'),
  (3, 'Mãe'),
  (4, 'Pai'),
  (5, 'Irmão'),
  (6, 'Advogado'),
  (7, 'Amante'),
  (8, 'Aliado'),
  (9, 'Vitima'),
  (10, 'Infrator'),
  (11, 'Testemunha'),
  (12, 'Vitma - Não Identificada'),
  (13, 'Infrator - Não identificado'),
  (14, 'Tio (a)'),
  (15, 'Avô'),
  (16, 'Avó'),
  (17, 'Padrasto'),
  (18, 'Madrasta'),
  (19, 'Primo (a)'),
  (20, 'Entiado (a)'),
  (21, 'Sogro (a)'),
  (22, 'Sobrinho (a)'),
  (23, 'Neto (a)');

-- ─── Marcas de celular com os ids do SIPOM ───────────────────────────
-- Nenhuma ficha guarda marca de celular ainda: a troca é só da lista.
DELETE FROM sipom.celular_marcas;
-- celular_marcas: 21 linhas
INSERT INTO sipom.celular_marcas (id, nome) VALUES
  (1, 'Apple'),
  (2, 'Samsung'),
  (3, 'Xiaomi'),
  (4, 'OnePlus'),
  (5, 'Motorola'),
  (6, 'Sony'),
  (7, 'Realme'),
  (8, 'Vivo'),
  (9, 'TCL'),
  (10, 'ZTE'),
  (11, 'Nokia'),
  (12, 'LG'),
  (13, 'Lenovo'),
  (14, 'Huawei'),
  (15, 'HTC'),
  (16, 'BLU'),
  (17, 'BlackBerry'),
  (18, 'ASUS'),
  (19, 'Alcatel'),
  (20, 'Acer'),
  (21, 'Outros');

GRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sipom.vinculos;
DELETE FROM sipom.celular_marcas;
INSERT INTO sipom.celular_marcas (id, nome) VALUES
  (1, 'Acer'),
  (2, 'Alcatel'),
  (3, 'Apple'),
  (4, 'ASUS'),
  (5, 'BlackBerry'),
  (6, 'BLU'),
  (7, 'HTC'),
  (8, 'Huawei'),
  (9, 'Lenovo'),
  (10, 'LG'),
  (11, 'Motorola'),
  (12, 'Nokia'),
  (13, 'OnePlus'),
  (14, 'Outros'),
  (15, 'Realme'),
  (16, 'Samsung'),
  (17, 'Sony'),
  (18, 'TCL'),
  (19, 'Vivo'),
  (20, 'Xiaomi'),
  (21, 'ZTE');
-- +goose StatementEnd

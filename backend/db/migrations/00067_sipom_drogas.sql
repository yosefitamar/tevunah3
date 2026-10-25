-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_drogas.py a partir do quarto dump do SIPOM
-- (2026-10-19: droga_tipos). Não editar à mão: regenere pelo script.


-- ─── Drogas com os ids do SIPOM ──────────────────────────────────────
-- Mesmos nomes e unidades da lista lida na tela (00059); só os ids mudam. A
-- quantidade gravada continua valendo (a unidade é a mesma).

-- antigo (00059) → SIPOM, pelo nome
CREATE TEMP TABLE droga_map (de integer PRIMARY KEY, para integer NOT NULL);
INSERT INTO droga_map (de, para) VALUES (1, 11), (2, 4), (3, 5), (4, 8), (5, 12), (6, 2), (7, 10), (8, 7), (9, 1), (10, 9), (11, 3), (12, 6);
ALTER TABLE app.ops_occurrence_drugs DROP CONSTRAINT IF EXISTS ops_occurrence_drugs_sipom_droga_id_fkey;
UPDATE app.ops_occurrence_drugs d SET sipom_droga_id = m.para
  FROM droga_map m WHERE d.sipom_droga_id = m.de;
UPDATE app.sipom_term_map t SET target_id = m.para, updated_at = now()
  FROM droga_map m WHERE t.field = 'droga' AND t.target_id = m.de;
DROP TABLE droga_map;

DELETE FROM sipom.drogas;
INSERT INTO sipom.drogas (id, nome, unidade) VALUES
  (1, 'Maconha', 'gramas (g)'),
  (2, 'Haxixe', 'gramas (g)'),
  (3, 'Skank', 'gramas (g)'),
  (4, 'Cocaína', 'gramas (g)'),
  (5, 'Crack', 'gramas (g)'),
  (6, 'Solvente', 'mililitros (ml)'),
  (7, 'Lsd', 'dose'),
  (8, 'Ecstasy/MDMA', 'comprimido'),
  (9, 'Quetamina', 'comprimido'),
  (10, 'Heroína', 'gramas (g)'),
  (11, 'Chá de Ayahuasca', 'mililitros (ml)'),
  (12, 'Fentanil', 'miligramas (mg)');
ALTER TABLE app.ops_occurrence_drugs ADD CONSTRAINT ops_occurrence_drugs_sipom_droga_id_fkey
  FOREIGN KEY (sipom_droga_id) REFERENCES sipom.drogas(id);

GRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SIPOM → antigo (00059)
CREATE TEMP TABLE droga_map (de integer PRIMARY KEY, para integer NOT NULL);
INSERT INTO droga_map (de, para) VALUES (11, 1), (4, 2), (5, 3), (8, 4), (12, 5), (2, 6), (10, 7), (7, 8), (1, 9), (9, 10), (3, 11), (6, 12);
ALTER TABLE app.ops_occurrence_drugs DROP CONSTRAINT IF EXISTS ops_occurrence_drugs_sipom_droga_id_fkey;
UPDATE app.ops_occurrence_drugs d SET sipom_droga_id = m.para
  FROM droga_map m WHERE d.sipom_droga_id = m.de;
UPDATE app.sipom_term_map t SET target_id = m.para, updated_at = now()
  FROM droga_map m WHERE t.field = 'droga' AND t.target_id = m.de;
DROP TABLE droga_map;
DELETE FROM sipom.drogas;
INSERT INTO sipom.drogas (id, nome, unidade) VALUES
  (1, 'Chá de Ayahuasca', 'mililitros (ml)'),
  (2, 'Cocaína', 'gramas (g)'),
  (3, 'Crack', 'gramas (g)'),
  (4, 'Ecstasy/MDMA', 'comprimido'),
  (5, 'Fentanil', 'miligramas (mg)'),
  (6, 'Haxixe', 'gramas (g)'),
  (7, 'Heroína', 'gramas (g)'),
  (8, 'Lsd', 'dose'),
  (9, 'Maconha', 'gramas (g)'),
  (10, 'Quetamina', 'comprimido'),
  (11, 'Skank', 'gramas (g)'),
  (12, 'Solvente', 'mililitros (ml)');
ALTER TABLE app.ops_occurrence_drugs ADD CONSTRAINT ops_occurrence_drugs_sipom_droga_id_fkey
  FOREIGN KEY (sipom_droga_id) REFERENCES sipom.drogas(id);
-- +goose StatementEnd

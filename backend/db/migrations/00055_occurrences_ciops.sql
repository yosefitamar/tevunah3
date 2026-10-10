-- +goose Up
-- +goose StatementBegin

-- ─── Ficha CIOPS como identidade da ocorrência ────────────────────────
-- Ocorrências passa a reunir o cadastro manual (app.incidents) e o que vem
-- do relatório operacional (app.ops_occurrences). As duas tabelas continuam
-- separadas — uma é curadoria do analista, a outra é cópia fiel do PDF e
-- origem do envio ao SIPOM — e a ficha CIOPS é o que as une: mesma ficha,
-- mesma ocorrência, uma linha só na listagem.
--
-- Para isso a ficha precisa ser única em cada lado. No relatório já é
-- (ops_occurrences_ciops_uniq); aqui entra a garantia do cadastro manual.

-- Forma de comparação da ficha: só letras e dígitos, em maiúsculas. Espaço,
-- hífen, ponto e caixa são variações de digitação, não fichas diferentes
-- ("m 2026-0705888" = "M20260705888").
CREATE OR REPLACE FUNCTION app.norm_ciops(txt text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
$$ SELECT upper(regexp_replace(txt, '[^A-Za-z0-9]', '', 'g')) $$;

GRANT EXECUTE ON FUNCTION app.norm_ciops(text) TO tevunah_app;

CREATE INDEX incidents_ciops_norm_idx
  ON app.incidents (app.norm_ciops(ciops_record))
  WHERE deleted_at IS NULL AND ciops_record <> '';
CREATE INDEX ops_occurrences_ciops_norm_idx
  ON app.ops_occurrences (app.norm_ciops(ciops_record))
  WHERE deleted_at IS NULL AND ciops_record <> '';

-- Unicidade no cadastro manual, por gatilho e não por índice único: a ficha
-- era texto livre sem restrição até aqui, e o acervo pode ter repetições
-- antigas que um índice único recusaria na criação. O gatilho barra toda
-- gravação nova (inclusão, troca de ficha, restauração de excluída) e deixa
-- as repetições antigas onde estão, para o analista resolver.
--
-- O advisory lock serializa duas gravações simultâneas da mesma ficha — sem
-- ele, as duas passariam pelo EXISTS antes de qualquer uma confirmar.
CREATE OR REPLACE FUNCTION app.incidents_ciops_guard() RETURNS trigger
  LANGUAGE plpgsql AS
$$
DECLARE
  k text := app.norm_ciops(NEW.ciops_record);
BEGIN
  IF k = '' OR NEW.deleted_at IS NOT NULL THEN
    RETURN NEW;
  END IF;
  IF TG_OP = 'UPDATE'
     AND OLD.deleted_at IS NULL
     AND app.norm_ciops(OLD.ciops_record) = k THEN
    RETURN NEW;
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('incidents_ciops:' || k, 0));
  IF EXISTS (
    SELECT 1 FROM app.incidents
     WHERE id <> NEW.id
       AND deleted_at IS NULL
       AND ciops_record <> ''
       AND app.norm_ciops(ciops_record) = k
  ) THEN
    RAISE EXCEPTION 'ficha CIOPS % já cadastrada', NEW.ciops_record
      USING ERRCODE = 'unique_violation', CONSTRAINT = 'incidents_ciops_uniq';
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER incidents_ciops_guard
  BEFORE INSERT OR UPDATE OF ciops_record, deleted_at ON app.incidents
  FOR EACH ROW EXECUTE FUNCTION app.incidents_ciops_guard();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS incidents_ciops_guard ON app.incidents;
DROP FUNCTION IF EXISTS app.incidents_ciops_guard();
DROP INDEX IF EXISTS app.ops_occurrences_ciops_norm_idx;
DROP INDEX IF EXISTS app.incidents_ciops_norm_idx;
DROP FUNCTION IF EXISTS app.norm_ciops(text);
-- +goose StatementEnd

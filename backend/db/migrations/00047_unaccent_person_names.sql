-- +goose Up
-- +goose StatementBegin

-- ─── Busca e gravação insensíveis a acento ────────────────────────────
--
-- Duas regras distintas, que se complementam:
--
--  1. GRAVAÇÃO: nomes de PESSOA (nome, alcunhas, nome da mãe) passam a ser
--     persistidos sem acento. O acervo vem de fontes que grafam o mesmo
--     indivíduo ora "JOSÉ", ora "JOSE" — sem uma forma canônica, homônimo
--     escapa da detecção e o mesmo suspeito vira dois dossiês. Organização,
--     lugar e veículo mantêm a grafia (razão social e logradouro têm valor
--     documental).
--
--  2. LEITURA: a busca dobra os dois lados por app.norm_txt(), então
--     "SÃO PAULO" e "SAO PAULO" chegam ao mesmo lugar mesmo nos kinds que
--     preservam o acento.
--
-- unaccent é uma extensão trusted desde o PG13; a migration roda com o
-- superusuário (MIGRATIONS_DATABASE_URL), então o CREATE passa.
CREATE EXTENSION IF NOT EXISTS "unaccent" WITH SCHEMA public;

-- A forma unaccent(text) é apenas STABLE (resolve o dicionário pelo
-- search_path). A forma de 2 argumentos é IMMUTABLE — é ela que permite usar
-- a normalização dentro de índice.
CREATE OR REPLACE FUNCTION app.unaccent_txt(txt text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
$$ SELECT public.unaccent('public.unaccent'::regdictionary, txt) $$;

-- Forma canônica de comparação: sem acento e em minúsculas. Usada por toda
-- busca textual (entidades e ocorrências) e pela detecção de homônimo.
CREATE OR REPLACE FUNCTION app.norm_txt(txt text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS
$$ SELECT lower(public.unaccent('public.unaccent'::regdictionary, txt)) $$;

GRANT EXECUTE ON FUNCTION app.unaccent_txt(text) TO tevunah_app;
GRANT EXECUTE ON FUNCTION app.norm_txt(text)     TO tevunah_app;

-- ─── Backfill: pessoas já cadastradas ─────────────────────────────────
-- A capitalização já foi normalizada pela 00041; aqui só cai o acento.

UPDATE app.entities
   SET name = app.unaccent_txt(name)
 WHERE kind = 'person'
   AND name IS DISTINCT FROM app.unaccent_txt(name);

UPDATE app.entity_persons
   SET mother_name = app.unaccent_txt(mother_name)
 WHERE mother_name IS DISTINCT FROM app.unaccent_txt(mother_name);

UPDATE app.entity_persons
   SET aliases = (SELECT COALESCE(array_agg(app.unaccent_txt(a) ORDER BY ord), '{}')
                    FROM unnest(aliases) WITH ORDINALITY AS t(a, ord))
 WHERE EXISTS (SELECT 1 FROM unnest(aliases) a
                WHERE a IS DISTINCT FROM app.unaccent_txt(a));

-- ─── Backfill: descrição das ocorrências em MAIÚSCULAS ────────────────
-- Mesma regra que a 00041 aplicou às entidades; o módulo de ocorrências
-- nasceu depois e ficou de fora.

UPDATE app.incidents
   SET description = upper(description)
 WHERE description IS DISTINCT FROM upper(description);

-- ─── Backfill: óbito legado ───────────────────────────────────────────
-- A importação do acervo antigo traduziu a flag is_dead numa tag livre
-- 'falecido' (a coluna deceased só nasceu na 00046). Sem esta conversão, a
-- sinalização de óbito da listagem ignoraria justamente os mortos herdados
-- do sistema anterior. A tag é preservada como rastro da origem.

UPDATE app.entity_persons p
   SET deceased = true
 WHERE NOT p.deceased
   AND EXISTS (SELECT 1 FROM app.entity_tags t
                WHERE t.entity_id = p.entity_id AND t.tag = 'falecido');

-- ─── Índices ──────────────────────────────────────────────────────────
-- O índice por lower(name) deixa de casar com o predicado da busca, que
-- agora normaliza acento junto. Trocado pelo equivalente sobre norm_txt.

DROP INDEX IF EXISTS app.entities_name_lower_idx;
CREATE INDEX entities_name_norm_idx
  ON app.entities (app.norm_txt(name)) WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS app.entities_name_norm_idx;
CREATE INDEX entities_name_lower_idx
  ON app.entities (lower(name)) WHERE deleted_at IS NULL;
DROP FUNCTION IF EXISTS app.norm_txt(text);
DROP FUNCTION IF EXISTS app.unaccent_txt(text);
-- Irreversível: os acentos originais dos nomes de pessoa e a capitalização
-- original das descrições de ocorrência não são preservados.
-- +goose StatementEnd

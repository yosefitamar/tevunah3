#!/usr/bin/env python3
"""Gera a migração 00067 a partir do quarto dump do SIPOM (recebido em
2026-10-19): a tabela droga_tipos, com os ids do SIPOM (droga_tipo_id da API
de integração). As drogas de 00059 foram numeradas por nós a partir da tela;
os nomes e as unidades são os mesmos, os ids não. Como os ids já estão
gravados (tradução das drogas das ocorrências e termos aprendidos), a
migração remapeia pelo nome antes de trocar a lista.

Uso:  python3 backend/db/sipom/gen_drogas.py ~/Downloads/"Dump20261019 (1).sql"
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gen_migrations import FOOTER, inserts, load, write  # noqa: E402
from gen_procedimento import DROGAS  # noqa: E402

HEADER = """-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_drogas.py a partir do quarto dump do SIPOM
-- (2026-10-19: droga_tipos). Não editar à mão: regenere pelo script.
"""


def remap(pairs, label):
    """SQL que troca os ids gravados de droga segundo pairs [(de, para)]."""
    values = ", ".join(f"({a}, {b})" for a, b in pairs)
    return f"""
-- {label}
CREATE TEMP TABLE droga_map (de integer PRIMARY KEY, para integer NOT NULL);
INSERT INTO droga_map (de, para) VALUES {values};
ALTER TABLE app.ops_occurrence_drugs DROP CONSTRAINT IF EXISTS ops_occurrence_drugs_sipom_droga_id_fkey;
UPDATE app.ops_occurrence_drugs d SET sipom_droga_id = m.para
  FROM droga_map m WHERE d.sipom_droga_id = m.de;
UPDATE app.sipom_term_map t SET target_id = m.para, updated_at = now()
  FROM droga_map m WHERE t.field = 'droga' AND t.target_id = m.de;
DROP TABLE droga_map;"""


def refill(rows):
    return "\n".join([
        "DELETE FROM sipom.drogas;",
        inserts("drogas", ["id", "nome", "unidade"], rows),
        "ALTER TABLE app.ops_occurrence_drugs ADD CONSTRAINT ops_occurrence_drugs_sipom_droga_id_fkey",
        "  FOREIGN KEY (sipom_droga_id) REFERENCES sipom.drogas(id);",
    ])


def main(dump):
    d = load(dump)
    novas = [{"id": r["id"], "nome": r["nome"].strip(), "unidade": r["unidade"].strip()} for r in d["droga_tipos"]]
    antigas = [{"id": i, "nome": n, "unidade": u} for i, n, u in DROGAS]
    by_name = {r["nome"]: r for r in novas}
    pairs = []
    for a in antigas:
        n = by_name.get(a["nome"])
        if n is None:
            sys.exit(f"droga sem correspondência no dump: {a['nome']}")
        if n["unidade"] != a["unidade"]:
            sys.exit(f"unidade mudou ({a['nome']}): {a['unidade']} → {n['unidade']} — a quantidade gravada não serve")
        pairs.append((a["id"], n["id"]))
    if len(novas) != len(antigas):
        sys.exit(f"o dump tem {len(novas)} drogas, a lista antiga {len(antigas)}")

    up = [HEADER, """
-- ─── Drogas com os ids do SIPOM ──────────────────────────────────────
-- Mesmos nomes e unidades da lista lida na tela (00059); só os ids mudam. A
-- quantidade gravada continua valendo (a unidade é a mesma).""",
          remap(pairs, "antigo (00059) → SIPOM, pelo nome"), "", refill(novas)]
    up.append("\nGRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;")
    down = remap([(b, a) for a, b in pairs], "SIPOM → antigo (00059)") + "\n" + refill(antigas)
    up.append(FOOTER.format(down=down))
    write("00067_sipom_drogas.sql", "\n".join(up))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("uso: gen_drogas.py <dump.sql>")
    main(os.path.expanduser(sys.argv[1]))

#!/usr/bin/env python3
"""Gera a migração 00066 a partir do terceiro dump do SIPOM (recebido em
2026-10-19): a tabela de vínculos de pessoa (vinculo_id da API de integração)
e as marcas de celular com os ids do SIPOM — as de 00059 foram numeradas por
nós a partir da tela. veiculo_tipos veio igual à de 00051 e não muda.

Uso:  python3 backend/db/sipom/gen_vinculos.py ~/Downloads/Dump20261019.sql
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gen_migrations import FOOTER, inserts, load, write  # noqa: E402
from gen_procedimento import CELULAR_MARCAS  # noqa: E402

HEADER = """-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_vinculos.py a partir do terceiro dump do
-- SIPOM (2026-10-19: vinculos e marca_celulares). Não editar à mão: regenere
-- pelo script.
"""


def main(dump):
    d = load(dump)
    vinculos = [{"id": r["id"], "nome": r["nome"]} for r in d["vinculos"]]
    marcas = [{"id": r["id"], "nome": r["nome"]} for r in d["marca_celulares"]]
    antigas = [{"id": i + 1, "nome": n} for i, n in enumerate(CELULAR_MARCAS)]

    up = [HEADER, """
-- ─── Vínculos de pessoa ──────────────────────────────────────────────
-- vinculo_id dos envolvidos na API do SIPOM. A ocorrência usa Vitima,
-- Infrator, Testemunha e os dois "não identificado"; os demais (parentesco,
-- advogado…) são vínculos entre pessoas no SIPOM.
CREATE TABLE sipom.vinculos (id integer PRIMARY KEY, nome text NOT NULL);
"""]
    up.append(f"-- vinculos: {len(vinculos)} linhas")
    up.append(inserts("vinculos", ["id", "nome"], vinculos))
    up.append("""
-- ─── Marcas de celular com os ids do SIPOM ───────────────────────────
-- Nenhuma ficha guarda marca de celular ainda: a troca é só da lista.
DELETE FROM sipom.celular_marcas;""")
    up.append(f"-- celular_marcas: {len(marcas)} linhas")
    up.append(inserts("celular_marcas", ["id", "nome"], marcas))
    up.append("\nGRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;")
    down = "DROP TABLE IF EXISTS sipom.vinculos;\nDELETE FROM sipom.celular_marcas;\n" + \
        inserts("celular_marcas", ["id", "nome"], antigas)
    up.append(FOOTER.format(down=down))
    write("00066_sipom_vinculos_celulares.sql", "\n".join(up))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("uso: gen_vinculos.py <dump.sql>")
    main(os.path.expanduser(sys.argv[1]))

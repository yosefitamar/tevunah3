#!/usr/bin/env python3
"""Gera a migração 00059 (procedimento e materiais do SIPOM) a partir do
segundo dump do SIPOM (delegacias e delegados, recebido em 2026-10-05) e das
listas lidas na tela do SIPOM em 2026-10-06, que não existem em dump nenhum
(drogas e marcas de celular — ver docs/sipom-materiais.md).

Uso:  python3 backend/db/sipom/gen_procedimento.py ~/Downloads/Dump20261005.sql
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gen_migrations import FOOTER, inserts, load, write  # noqa: E402

HEADER = """-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_procedimento.py a partir do segundo dump
-- do SIPOM (2026-10-05: delegacias e delegados) e das listas lidas na tela
-- do SIPOM em 2026-10-06 (drogas e marcas de celular, que não vêm em dump).
-- Não editar à mão: regenere pelo script.
"""

# Modal "Material" > Droga: o nome na tela carrega a unidade ("Maconha -
# gramas (g)"); aqui ficam separados. Não há tabela no SIPOM à vista: o id é
# nosso, e o envio leva o texto como a tela mostra (nome + unidade).
DROGAS = [
    (1, "Chá de Ayahuasca", "mililitros (ml)"),
    (2, "Cocaína", "gramas (g)"),
    (3, "Crack", "gramas (g)"),
    (4, "Ecstasy/MDMA", "comprimido"),
    (5, "Fentanil", "miligramas (mg)"),
    (6, "Haxixe", "gramas (g)"),
    (7, "Heroína", "gramas (g)"),
    (8, "Lsd", "dose"),
    (9, "Maconha", "gramas (g)"),
    (10, "Quetamina", "comprimido"),
    (11, "Skank", "gramas (g)"),
    (12, "Solvente", "mililitros (ml)"),
]

# Modal "Material" > Celular > Marca, na ordem da tela.
CELULAR_MARCAS = [
    "Acer", "Alcatel", "Apple", "ASUS", "BlackBerry", "BLU", "HTC", "Huawei", "Lenovo",
    "LG", "Motorola", "Nokia", "OnePlus", "Outros", "Realme", "Samsung", "Sony", "TCL",
    "Vivo", "Xiaomi", "ZTE",
]


def main(dump):
    d = load(dump)
    delegacias = [{"id": r["id"], "nome": r["nome"]} for r in d["delegacias"]]
    delegados = [{"id": r["id"], "nome": r["nome"], "deletado_em": r.get("deletado_em")} for r in d["delegados"]]
    drogas = [{"id": i, "nome": n, "unidade": u} for i, n, u in DROGAS]
    marcas = [{"id": i + 1, "nome": n} for i, n in enumerate(CELULAR_MARCAS)]

    up = [HEADER, """
-- ─── Procedimento ────────────────────────────────────────────────────
-- Delegacia e delegado do procedimento (aba Procedimentos do SIPOM). O nome
-- da delegacia traz o código da unidade na frente ("201-DELEGACIA
-- METROPOLITANA DE CAUCAIA"): é o mesmo código que abre o número do
-- procedimento no relatório operacional ("939-7635/2026").
CREATE TABLE sipom.delegacias (id integer PRIMARY KEY, nome text NOT NULL);
CREATE TABLE sipom.delegados (id integer PRIMARY KEY, nome text NOT NULL, deletado_em timestamp);

-- ─── Materiais sem tabela no SIPOM ───────────────────────────────────
CREATE TABLE sipom.drogas (id integer PRIMARY KEY, nome text NOT NULL, unidade text NOT NULL);
CREATE TABLE sipom.celular_marcas (id integer PRIMARY KEY, nome text NOT NULL);
"""]
    for t, cols, rows in [
        ("delegacias", ["id", "nome"], delegacias),
        ("delegados", ["id", "nome", "deletado_em"], delegados),
        ("drogas", ["id", "nome", "unidade"], drogas),
        ("celular_marcas", ["id", "nome"], marcas),
    ]:
        up.append(f"-- {t}: {len(rows)} linhas")
        up.append(inserts(t, cols, rows))
    up.append("\nGRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;")
    up.append(FOOTER.format(down="\n".join(
        f"DROP TABLE IF EXISTS sipom.{t};" for t in ["celular_marcas", "drogas", "delegados", "delegacias"])))
    write("00059_sipom_procedimento_materiais.sql", "\n".join(up))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("uso: gen_procedimento.py <dump.sql>")
    main(os.path.expanduser(sys.argv[1]))

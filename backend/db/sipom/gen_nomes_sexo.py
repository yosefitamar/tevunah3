#!/usr/bin/env python3
"""Gera a migração 00068: frequência de cada primeiro nome por sexo, do Censo
2010 do IBGE (dataset "genero-nomes" do Brasil.IO). É dela que sai a sugestão
de sexo do envolvido para o SIPOM quando o dossiê não diz.

Os números ficam como no censo; o corte (frequência mínima e proporção) é
decidido no backend (internal/sipom/sexo.go).

Uso:  curl -LO https://data.brasil.io/dataset/genero-nomes/nomes.csv.gz
      python3 backend/db/sipom/gen_nomes_sexo.py nomes.csv.gz
"""
import csv
import gzip
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from gen_migrations import FOOTER, write  # noqa: E402

HEADER = """-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_nomes_sexo.py a partir do dataset
-- "genero-nomes" do Brasil.IO (Censo 2010 do IBGE). Não editar à mão:
-- regenere pelo script.

-- ─── Primeiro nome × sexo ────────────────────────────────────────────
-- Quantas pessoas com o primeiro nome eram do sexo feminino e do masculino
-- no censo. Sugere o sexo do envolvido sem dossiê (o SIPOM exige sexo de
-- toda pessoa identificada); o analista confirma.
CREATE TABLE app.nomes_sexo (
  nome      text PRIMARY KEY, -- maiúsculo, sem acento, como no censo
  feminino  integer NOT NULL,
  masculino integer NOT NULL
);
GRANT SELECT ON app.nomes_sexo TO tevunah_app;
"""


def lit(s):
    return "'" + s.replace("'", "''") + "'"


def main(path):
    rows = []
    with gzip.open(path, "rt", encoding="utf-8") as f:
        for r in csv.DictReader(f):
            nome = r["first_name"].strip().upper()
            if not nome:
                continue
            rows.append((nome, int(r["frequency_female"] or 0), int(r["frequency_male"] or 0)))
    rows.sort()
    out = [HEADER, f"-- {len(rows)} nomes"]
    for i in range(0, len(rows), 1000):
        vals = ",\n  ".join(f"({lit(n)}, {f}, {m})" for n, f, m in rows[i:i + 1000])
        out.append(f"INSERT INTO app.nomes_sexo (nome, feminino, masculino) VALUES\n  {vals};")
    sql = "\n".join(out) + FOOTER.format(down="DROP TABLE IF EXISTS app.nomes_sexo;")
    write("00068_nomes_sexo.sql", sql)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("uso: gen_nomes_sexo.py <nomes.csv.gz>")
    main(os.path.expanduser(sys.argv[1]))

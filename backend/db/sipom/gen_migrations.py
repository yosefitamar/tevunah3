#!/usr/bin/env python3
"""Gera as migrações do catálogo SIPOM a partir do dump fornecido pelo SIPOM.

O SIPOM (sipom.pm.ce.gov.br) é o destino do envio das ocorrências. O dump
(MySQL, só tabelas de referência, recebido em 2026-10-02) não será reenviado:
as migrações geradas aqui são a cópia versionada. Este script fica para
documentar a origem e permitir regenerar se um dump novo aparecer.

Uso:  python3 backend/db/sipom/gen_migrations.py ~/Downloads/Dump20261002.sql

Regras:
  - IDs e textos oficiais preservados: são o que vai no envio.
  - Território restrito ao Ceará (estado_id = 5): cidade e bairro.
  - natureza_fatos ganha `rotulo`, grafia sanitizada só para exibição.
  - marcasmodelos (tabela FIPE/DENATRAN, ~40 mil linhas) vai em migração própria.
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
MIGRATIONS = os.path.join(HERE, "..", "migrations")
CE = 5  # estado_id do Ceará no SIPOM


# ─── Leitura do dump ───

def parse_values(s):
    """Tuplas de um INSERT ... VALUES (...),(...) no formato do mysqldump."""
    rows, row, i, n = [], [], 0, len(s)
    while i < n:
        c = s[i]
        if c == "(":
            row = []
            i += 1
        elif c == ")":
            rows.append(row)
            i += 1
        elif c in ", ;\n":
            i += 1
        elif c == "'":
            i += 1
            buf = []
            while i < n:
                ch = s[i]
                if ch == "\\":
                    nx = s[i + 1]
                    buf.append({"n": "\n", "r": "\r", "t": "\t", "0": "\0"}.get(nx, nx))
                    i += 2
                elif ch == "'":
                    if i + 1 < n and s[i + 1] == "'":
                        buf.append("'")
                        i += 2
                    else:
                        i += 1
                        break
                else:
                    buf.append(ch)
                    i += 1
            row.append("".join(buf))
        else:
            tok = re.match(r"NULL|-?\d+(\.\d+)?", s[i:]).group(0)
            row.append(None if tok == "NULL" else (float(tok) if "." in tok else int(tok)))
            i += len(tok)
    return rows


def load(path):
    text = open(path, encoding="utf-8").read()
    cols, data = {}, {}
    for m in re.finditer(r"CREATE TABLE `(\w+)` \((.*?)\n\) ENGINE", text, re.S):
        cols[m.group(1)] = re.findall(r"^\s+`(\w+)`", m.group(2), re.M)
    for m in re.finditer(r"^INSERT INTO `(\w+)` VALUES (.*?);$", text, re.M | re.S):
        t = m.group(1)
        data.setdefault(t, []).extend(dict(zip(cols[t], r)) for r in parse_values(m.group(2)))
    return data


# ─── Rótulos sanitizados das naturezas ───
# O SIPOM grava sem acento, com erros de digitação e espaçamento irregular.
# O rótulo corrige isso para exibição; o envio usa sempre o id.

WORDS = {
    "HOMICIDIO": "HOMICÍDIO", "LESAO": "LESÃO", "LESOES": "LESÕES", "VEICULO": "VEÍCULO",
    "TRAFICO": "TRÁFICO", "NAO": "NÃO", "CALUNIA": "CALÚNIA", "DIFAMACAO": "DIFAMAÇÃO",
    "INJURIA": "INJÚRIA", "CARCERE": "CÁRCERE", "VIOLACAO": "VIOLAÇÃO", "DOMICILIO": "DOMICÍLIO",
    "EXTORSAO": "EXTORSÃO", "APROPRIACAO": "APROPRIAÇÃO", "INDEBITA": "INDÉBITA",
    "RECEPTACAO": "RECEPTAÇÃO", "SEDUCAO": "SEDUÇÃO", "CORRUPCAO": "CORRUPÇÃO", "RACA": "RAÇA",
    "TRANSITO": "TRÂNSITO", "PERICLITACAO": "PERICLITAÇÃO", "SAUDE": "SAÚDE",
    "LATROCINIO": "LATROCÍNIO", "PATRIMONIO": "PATRIMÔNIO", "ORGANIZACAO": "ORGANIZAÇÃO",
    "FAMILIA": "FAMÍLIA", "PUBLICA": "PÚBLICA", "FE": "FÉ", "ADMINISTRACAO": "ADMINISTRAÇÃO",
    "CONTRAVENCAO": "CONTRAVENÇÃO", "TRIBUTARIA": "TRIBUTÁRIA", "SUICIDIO": "SUICÍDIO",
    "DIRECAO": "DIREÇÃO", "EXPLORACAO": "EXPLORAÇÃO", "ECONOMICA": "ECONÔMICA",
    "INTERCEPTACAO": "INTERCEPTAÇÃO", "OCULTACAO": "OCULTAÇÃO", "OMISSAO": "OMISSÃO",
    "COMERCIO": "COMÉRCIO", "ILICITO": "ILÍCITO", "VIOLENCIA": "VIOLÊNCIA",
    "DOMESTICA": "DOMÉSTICA", "VULNERAVEL": "VULNERÁVEL", "RESTRICAO": "RESTRIÇÃO",
    "VITIMA": "VÍTIMA", "RESPONSABILIBDADE": "RESPONSABILIDADE",
}

FIX = {
    6: "FURTO (OUTROS)",
    86: "CRIME PREVISTO NO ESTATUTO DA CRIANÇA E DO ADOLESCENTE",
    90: "CRIME PREVISTO NA LEI DE LICITAÇÕES",
    92: "CRIME PREVISTO NA LEI DE RESPONSABILIDADE FISCAL",
    101: "CRIME CONTRA A ADMINISTRAÇÃO PÚBLICA (PARCELAMENTO DO SOLO URBANO)",
    105: "PORTE ILEGAL DE ARMA DE FOGO (NOVA LEGISLAÇÃO)",
    117: "LAVAGEM OU OCULTAÇÃO DE BENS, DIREITOS E VALORES",
    126: "CRIME PREVISTO NA LEI 7.347/85, ART. 10",
    127: "CRIME PREVISTO NA LEI 9.504/97 (NORMAS PARA ELEIÇÃO)",
    136: "USUÁRIOS OU DEPENDENTES DE DROGAS",
    150: "ATENTADO CONTRA A SEGURANÇA DE TRANSPORTE MARÍTIMO, FLUVIAL OU AÉREO",
}


def rotulo(id_, nome):
    if id_ in FIX:
        return FIX[id_]
    s = re.sub(r"\s+", " ", nome.strip().upper())
    s = re.sub(r"\(\s*", "(", s)
    s = re.sub(r"\s*\)", ")", s)
    s = re.sub(r"(\w)\(", r"\1 (", s)
    # Palavra a palavra; "(LATROCINIO)" também casa.
    return re.sub(r"[A-ZÀ-Ý]+", lambda m: WORDS.get(m.group(0), m.group(0)), s)


# ─── SQL ───

def lit(v):
    if v is None:
        return "NULL"
    if isinstance(v, (int, float)):
        return str(v)
    return "'" + str(v).replace("'", "''") + "'"


def inserts(table, columns, rows, chunk=500):
    out = []
    for i in range(0, len(rows), chunk):
        vals = ",\n  ".join("(" + ", ".join(lit(r[c]) for c in columns) + ")" for r in rows[i:i + chunk])
        out.append(f"INSERT INTO sipom.{table} ({', '.join(columns)}) VALUES\n  {vals};")
    return "\n".join(out)


# Tabelas, colunas copiadas e DDL. Nomes seguem o SIPOM para o mapeamento
# ser direto; só os carimbos criado/atualizado ficam de fora. A ordem
# respeita as chaves estrangeiras.
TABLES = [
    ("natureza_fatos", ["id", "nome", "rotulo"],
     "id integer PRIMARY KEY, nome text NOT NULL, rotulo text NOT NULL"),
    ("comandos", ["id", "nome", "abreviado", "risp", "deletado_em"],
     "id integer PRIMARY KEY, nome text NOT NULL, abreviado text NOT NULL, risp text, deletado_em timestamp"),
    ("batalhoes", ["id", "cp_id", "nome", "abreviado", "deletado_em"],
     "id integer PRIMARY KEY, cp_id integer NOT NULL REFERENCES sipom.comandos(id), nome text NOT NULL, "
     "abreviado text NOT NULL, deletado_em timestamp"),
    ("companhias", ["id", "cp_id", "bpm_id", "nome", "abreviado", "deletado_em"],
     "id integer PRIMARY KEY, cp_id integer, bpm_id integer, nome text NOT NULL, abreviado text NOT NULL, "
     "deletado_em timestamp"),
    ("cidade", ["id", "codigo", "nome"],
     "id integer PRIMARY KEY, codigo integer, nome text NOT NULL"),
    ("bairro", ["id", "cidade_id", "nome", "deletado_em"],
     "id integer PRIMARY KEY, cidade_id integer NOT NULL REFERENCES sipom.cidade(id), nome text NOT NULL, "
     "deletado_em timestamp"),
    ("companhia_atuacao", ["id", "companhia_id", "bairro_id", "cidade_id"],
     "id integer PRIMARY KEY, companhia_id integer NOT NULL REFERENCES sipom.companhias(id), "
     "bairro_id integer REFERENCES sipom.bairro(id), cidade_id integer REFERENCES sipom.cidade(id)"),
    ("procedimentos", ["id", "nome"], "id integer PRIMARY KEY, nome text NOT NULL"),
    ("material_tipos", ["id", "nome"], "id integer PRIMARY KEY, nome text NOT NULL"),
    ("arma_tipos", ["id", "nome", "requerido", "deletado_em"],
     "id integer PRIMARY KEY, nome text NOT NULL, requerido smallint NOT NULL, deletado_em timestamp"),
    ("arma_calibres", ["id", "nome", "deletado_em"],
     "id integer PRIMARY KEY, nome text NOT NULL, deletado_em timestamp"),
    ("arma_marcas", ["id", "nome", "deletado_em"],
     "id integer PRIMARY KEY, nome text NOT NULL, deletado_em timestamp"),
    ("veiculo_tipos", ["id", "codigo_tipos_veiculos", "nome"],
     "id integer PRIMARY KEY, codigo_tipos_veiculos integer NOT NULL, nome text NOT NULL"),
    ("veiculo_cores", ["id", "codigo_cor", "nome"],
     "id integer PRIMARY KEY, codigo_cor integer NOT NULL, nome text NOT NULL"),
    ("orcrims", ["id", "nome", "abreviatura", "deletado_em"],
     "id integer PRIMARY KEY, nome text NOT NULL, abreviatura text NOT NULL, deletado_em timestamp"),
    ("postos_graduacoes", ["id", "nome", "ordem", "abreviado", "tipo"],
     "id integer PRIMARY KEY, nome text NOT NULL, ordem integer, abreviado text NOT NULL, tipo smallint NOT NULL"),
    ("policiamentos_tipos", ["id", "nome"], "id integer PRIMARY KEY, nome text NOT NULL"),
    ("policiamentos_funcoes", ["id", "nome"], "id integer PRIMARY KEY, nome text NOT NULL"),
    ("policiamentos_tipos_funcoes", ["id", "policiamento_tipo_id", "policiamento_funcao_id"],
     "id integer PRIMARY KEY, policiamento_tipo_id integer NOT NULL REFERENCES sipom.policiamentos_tipos(id), "
     "policiamento_funcao_id integer NOT NULL REFERENCES sipom.policiamentos_funcoes(id)"),
    ("atributos_tipos", ["id", "nome"], "id integer PRIMARY KEY, nome text NOT NULL"),
    ("atributos_subtipos", ["id", "atributo_tipo_id", "nome"],
     "id integer PRIMARY KEY, atributo_tipo_id integer NOT NULL REFERENCES sipom.atributos_tipos(id), "
     "nome text NOT NULL"),
]

HEADER = """-- +goose Up
-- +goose StatementBegin

-- GERADO por backend/db/sipom/gen_migrations.py a partir do dump do SIPOM
-- (2026-10-02). Não editar à mão: regenere pelo script.
"""

FOOTER = """
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
{down}
-- +goose StatementEnd
"""


def main(dump):
    d = load(dump)
    ce_cities = {c["id"] for c in d["cidade"] if c["estado_id"] == CE}
    d["cidade"] = [c for c in d["cidade"] if c["id"] in ce_cities]
    d["bairro"] = [b for b in d["bairro"] if b["cidade_id"] in ce_cities]
    for r in d["natureza_fatos"]:
        r["rotulo"] = rotulo(r["id"], r["nome"])
    for t, cols, _ in TABLES:
        for r in d[t]:
            for c in cols:
                r.setdefault(c, None)

    up = [HEADER, """
-- ─── Catálogo do SIPOM ────────────────────────────────────────────────
-- Cópia das tabelas de referência do SIPOM, com os ids originais: é o que o
-- envio das ocorrências informa (natureza, área da unidade, OPM, materiais,
-- composição). Nomes de tabela e coluna seguem o SIPOM. Só leitura para a
-- aplicação: o catálogo pertence ao destino.
CREATE SCHEMA IF NOT EXISTS sipom;
GRANT USAGE ON SCHEMA sipom TO tevunah_app;
"""]
    for t, _, ddl in TABLES:
        up.append(f"CREATE TABLE sipom.{t} ({ddl});")
    up += [
        "CREATE INDEX bairro_cidade_idx ON sipom.bairro (cidade_id);",
        "CREATE INDEX companhia_atuacao_bairro_idx ON sipom.companhia_atuacao (bairro_id);",
        "CREATE INDEX companhia_atuacao_cidade_idx ON sipom.companhia_atuacao (cidade_id);",
        "",
    ]
    for t, cols, _ in TABLES:
        up.append(f"-- {t}: {len(d[t])} linhas")
        up.append(inserts(t, cols, d[t]))
    up.append("\nGRANT SELECT ON ALL TABLES IN SCHEMA sipom TO tevunah_app;")
    up.append(FOOTER.format(down="DROP SCHEMA IF EXISTS sipom CASCADE;"))
    write("00051_sipom_catalog.sql", "\n".join(up))

    mm = [{"id": r["id"], "codigo": r["codigo"], "descricao": r["desc"]} for r in d["marcasmodelos"]]
    write("00052_sipom_marcasmodelos.sql", "\n".join([
        HEADER,
        "-- Marca/modelo de veículo (código FIPE/DENATRAN) usado no material Veículo.",
        "CREATE TABLE sipom.marcasmodelos (id integer PRIMARY KEY, codigo integer NOT NULL, descricao text);",
        "CREATE INDEX marcasmodelos_codigo_idx ON sipom.marcasmodelos (codigo);",
        "",
        f"-- marcasmodelos: {len(mm)} linhas",
        inserts("marcasmodelos", ["id", "codigo", "descricao"], mm, 1000),
        "\nGRANT SELECT ON sipom.marcasmodelos TO tevunah_app;",
        FOOTER.format(down="DROP TABLE IF EXISTS sipom.marcasmodelos;"),
    ]))


def write(name, sql):
    path = os.path.normpath(os.path.join(MIGRATIONS, name))
    with open(path, "w", encoding="utf-8") as f:
        f.write(sql)
    print(f"{path}: {os.path.getsize(path):,} bytes")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("uso: gen_migrations.py <dump.sql>")
    main(os.path.expanduser(sys.argv[1]))

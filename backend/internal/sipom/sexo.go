package sipom

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
)

// Sugestão de sexo do envolvido sem dossiê. O SIPOM exige o sexo de toda
// pessoa identificada e o relatório não o traz. A sugestão não vai ao SIPOM
// sozinha: o analista confirma (uma a uma ou em lote, na fila).
//
// Fontes, em ordem:
//  1. o primeiro nome no Censo 2010 (app.nomes_sexo), quando o nome é comum
//     o bastante e quase só de um sexo;
//  2. o que os analistas já marcaram para o primeiro nome (termo sexo_nome);
//  3. o histórico no feminino ("conduzida", "a mesma"…), só para o único
//     identificado da ficha, que é acusado. O masculino não serve de pista:
//     o histórico usa "o indivíduo", "os conduzidos" também para mulheres.

// Fontes da sugestão.
const (
	SexoFonteNome      = "nome"
	SexoFonteAprendido = "aprendido"
	SexoFonteHistorico = "historico"
)

// Corte do censo: o nome precisa de ao menos nomeMinimo pessoas e de
// nomeProporcao delas num sexo só.
const (
	nomeMinimo    = 20
	nomeProporcao = 0.95
)

// NameSex é a frequência por sexo de cada primeiro nome no censo.
type NameSex struct {
	byName map[string][2]int32 // [feminino, masculino]
}

// LoadNameSex lê app.nomes_sexo (~100 mil nomes).
func LoadNameSex(ctx context.Context, db *sql.DB) (*NameSex, error) {
	rows, err := db.QueryContext(ctx, `SELECT nome, feminino, masculino FROM app.nomes_sexo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	n := &NameSex{byName: make(map[string][2]int32, 110_000)}
	for rows.Next() {
		var nome string
		var f, m int32
		if err := rows.Scan(&nome, &f, &m); err != nil {
			return nil, err
		}
		n.byName[key(nome)] = [2]int32{f, m}
	}
	return n, rows.Err()
}

// Guess devolve o sexo do primeiro nome pelo censo (0 = nome ausente, raro
// ou usado pelos dois sexos).
func (n *NameSex) Guess(first string) int {
	if n == nil {
		return 0
	}
	c, ok := n.byName[key(first)]
	if !ok {
		return 0
	}
	f, m := float64(c[0]), float64(c[1])
	switch total := f + m; {
	case total < nomeMinimo:
		return 0
	case f/total >= nomeProporcao:
		return SexoFeminino
	case m/total >= nomeProporcao:
		return SexoMasculino
	}
	return 0
}

// FirstName é o primeiro nome, a chave da sugestão e do termo aprendido.
func FirstName(name string) string {
	if f := strings.Fields(key(name)); len(f) > 0 {
		return f[0]
	}
	return ""
}

// feminineRE: o histórico se refere a alguém no feminino. Fica de fora o que
// também cabe a coisas ("abordada" — a moto) ou a outra pessoa ("a vítima").
var feminineRE = regexp.MustCompile(
	`\b(acusada|conduzida|autuada|flagranteada|infratora|indiciada|detida|capturada|presa)s?\b|\ba mesma\b`)

// SexoInput é o envolvido como a sugestão o vê.
type SexoInput struct {
	Name string
	Role string // ACUSADO | VÍTIMA | TESTEMUNHA
	// Narrative é o histórico da ocorrência; Single: o envolvido é o único
	// identificado da ficha.
	Narrative string
	Single    bool
}

// SuggestSexo sugere o sexo do envolvido (0 = sem sugestão) e diz de onde
// veio.
func SuggestSexo(in SexoInput, names *NameSex, terms *TermMap) (int, string) {
	first := FirstName(in.Name)
	if first == "" || Unidentified(in.Name) {
		return 0, ""
	}
	if s := names.Guess(first); s != 0 {
		return s, SexoFonteNome
	}
	if id, ok := terms.Lookup(TermSexoNome, first); ok && (id == SexoMasculino || id == SexoFeminino) {
		return id, SexoFonteAprendido
	}
	if in.Single && in.Role == "ACUSADO" && feminineRE.MatchString(key(in.Narrative)) {
		return SexoFeminino, SexoFonteHistorico
	}
	return 0, ""
}

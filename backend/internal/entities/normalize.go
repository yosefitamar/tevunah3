package entities

import "strings"

// Normalização de nomes de pessoa.
//
// Nome, alcunhas e nome da mãe são gravados sem acento. O acervo vem de
// fontes que grafam o mesmo indivíduo ora "JOSÉ", ora "JOSE": sem uma forma
// canônica na escrita, a detecção de homônimo não pega os dois e o mesmo
// suspeito vira dois dossiês. Organização, lugar e veículo ficam de fora —
// razão social e logradouro têm valor documental na grafia original.
//
// A LEITURA não depende disto: as buscas normalizam os dois lados via
// app.norm_txt() no Postgres (migration 00047), então acento digitado
// encontra registro sem acento em qualquer kind.

// accentFolds cobre Latin-1 Suplementar e Latin Estendido-A, alinhado com o
// dicionário do unaccent do Postgres para os caracteres que aparecem em
// nomes. Runas fora da tabela passam intactas.
var accentFolds = map[rune]string{
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A", 'Ā': "A", 'Ă': "A", 'Ą': "A",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'Æ': "AE", 'æ': "ae",
	'Ç': "C", 'Ć': "C", 'Ĉ': "C", 'Ċ': "C", 'Č': "C",
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'Ð': "D", 'Ď': "D", 'Đ': "D", 'ð': "d", 'ď': "d", 'đ': "d",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E", 'Ē': "E", 'Ĕ': "E", 'Ė': "E", 'Ę': "E", 'Ě': "E",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'Ĝ': "G", 'Ğ': "G", 'Ġ': "G", 'Ģ': "G", 'ĝ': "g", 'ğ': "g", 'ġ': "g", 'ģ': "g",
	'Ĥ': "H", 'Ħ': "H", 'ĥ': "h", 'ħ': "h",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I", 'Ĩ': "I", 'Ī': "I", 'Ĭ': "I", 'Į': "I", 'İ': "I",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'Ĵ': "J", 'ĵ': "j", 'Ķ': "K", 'ķ': "k",
	'Ĺ': "L", 'Ļ': "L", 'Ľ': "L", 'Ł': "L", 'ĺ': "l", 'ļ': "l", 'ľ': "l", 'ł': "l",
	'Ñ': "N", 'Ń': "N", 'Ņ': "N", 'Ň': "N", 'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O", 'Ō': "O", 'Ŏ': "O", 'Ő': "O",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'Œ': "OE", 'œ': "oe",
	'Ŕ': "R", 'Ŗ': "R", 'Ř': "R", 'ŕ': "r", 'ŗ': "r", 'ř': "r",
	'Ś': "S", 'Ŝ': "S", 'Ş': "S", 'Š': "S", 'ś': "s", 'ŝ': "s", 'ş': "s", 'š': "s",
	'ß': "ss",
	'Ţ': "T", 'Ť': "T", 'Ŧ': "T", 'ţ': "t", 'ť': "t", 'ŧ': "t",
	'Þ': "TH", 'þ': "th",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U", 'Ũ': "U", 'Ū': "U", 'Ŭ': "U", 'Ů': "U", 'Ű': "U", 'Ų': "U",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'Ŵ': "W", 'ŵ': "w",
	'Ý': "Y", 'Ŷ': "Y", 'Ÿ': "Y", 'ý': "y", 'ŷ': "y", 'ÿ': "y",
	'Ź': "Z", 'Ż': "Z", 'Ž': "Z", 'ź': "z", 'ż': "z", 'ž': "z",
}

// stripAccents devolve a string com os diacríticos removidos.
func stripAccents(s string) string {
	// Caminho rápido: a esmagadora maioria dos nomes já chega em ASCII.
	ascii := true
	for _, r := range s {
		if r > 127 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if sub, ok := accentFolds[r]; ok {
			b.WriteString(sub)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// sameNormalized compara dois textos ignorando caixa, acento e espaços nas
// pontas — a mesma equivalência que app.norm_txt() aplica no Postgres.
func sameNormalized(a, b string) bool {
	return strings.EqualFold(
		stripAccents(strings.TrimSpace(a)),
		stripAccents(strings.TrimSpace(b)),
	)
}

// entityName normaliza o nome da tabela base conforme o kind: pessoa perde
// o acento, os demais só sobem para MAIÚSCULAS.
func entityName(k Kind, name string) string {
	if k == KindPerson {
		return personText(name)
	}
	return upperTrim(name)
}

// entityNamePtr é a versão pointer-aware de entityName (nil = campo ausente
// no patch).
func entityNamePtr(k Kind, name *string) *string {
	if k == KindPerson {
		return personTextPtr(name)
	}
	return upperTrimPtr(name)
}

// personText normaliza um campo textual de pessoa: MAIÚSCULAS, sem espaços
// nas pontas e sem acento.
func personText(s string) string {
	return stripAccents(upperTrim(s))
}

// personTextPtr é a versão pointer-aware: nil continua nil (campo ausente
// num update parcial).
func personTextPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := personText(*p)
	return &v
}

// personTextSlice aplica personText a cada elemento, descartando vazios.
// nil continua nil.
func personTextSlice(ss []string) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if v := personText(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

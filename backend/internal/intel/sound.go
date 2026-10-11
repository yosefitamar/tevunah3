package intel

import "strings"

// Comparação de nomes próprios (pessoa, rua, bairro) tolerante ao que o
// digitador costuma errar. As funções recebem a palavra já normalizada
// (Normalize): minúscula e sem acento.

var soundPairs = strings.NewReplacer(
	"ph", "f", "th", "t", "ch", "x", "sh", "x", "lh", "li", "nh", "ni",
	"ç", "s", "ei", "e", "ou", "o", "qu", "c", "ck", "c",
)

// SoundKey reduz a palavra ao som, na medida do que se troca em nome
// português: s/z/ss/ç, i/y, v/w, c/k/q, h mudo, letra dobrada, "ei"/"e",
// "ou"/"o" e o "l" que soa "u" no fim da sílaba ("Pial"/"Piau",
// "Multirão"/"Mutirão").
func SoundKey(w string) string {
	rs := []rune(soundPairs.Replace(w))
	var b strings.Builder
	var last rune
	for i, r := range rs {
		if r == 'l' && (i == len(rs)-1 || !strings.ContainsRune("aeiouhl", rs[i+1])) {
			r = 'u'
		}
		switch r {
		case 'z':
			r = 's'
		case 'y':
			r = 'i'
		case 'w':
			r = 'v'
		case 'k', 'q':
			r = 'c'
		case 'h':
			continue
		}
		if r != last {
			b.WriteRune(r)
		}
		last = r
	}
	return b.String()
}

// NearSound: as palavras soam igual, ou o som difere em uma letra (troca,
// falta, sobra ou inversão). Palavra curta só casa pelo som exato — "Lima" e
// "Lira" são nomes diferentes.
func NearSound(a, b string) bool {
	ka, kb := SoundKey(a), SoundKey(b)
	if ka == kb {
		return true
	}
	if len(ka) < 5 || len(kb) < 5 {
		return false
	}
	return EditDistance(ka, kb) <= 1
}

// EditDistance é a distância de Damerau-Levenshtein (a inversão de duas
// letras vizinhas conta um erro só).
func EditDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}

// Grau de semelhança entre dois nomes de pessoa.
const (
	NameNone  = iota // nomes diferentes
	NameNear         // mesmas palavras, a menos de uma letra em alguma
	NameSound        // soam igual ("Sousa"/"Souza", "Thiago"/"Tiago")
	NameExact        // iguais, a menos de caixa e acento
)

// Conectivos que não distinguem um nome do outro.
var nameStop = map[string]bool{"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true}

// NameTokens são as palavras do nome que contam na comparação.
func NameTokens(s string) []string {
	var out []string
	for _, t := range strings.Fields(Normalize(s)) {
		if !nameStop[t] {
			out = append(out, t)
		}
	}
	return out
}

// NameMatch compara dois nomes completos de pessoa, palavra a palavra e na
// ordem: "Maria de Sousa Lima" e "MARIA DE SOUZA LIMA" são a mesma pessoa
// para o digitador. Nome com palavra a mais ou a menos não casa — o
// abreviado pode ser de outra pessoa. Tolera uma letra errada em, no máximo,
// metade das palavras.
func NameMatch(a, b string) int {
	ta, tb := NameTokens(a), NameTokens(b)
	if len(ta) == 0 || len(ta) != len(tb) {
		return NameNone
	}
	level, near := NameExact, 0
	for i := range ta {
		switch {
		case ta[i] == tb[i]:
		case SoundKey(ta[i]) == SoundKey(tb[i]):
			level = min(level, NameSound)
		case NearSound(ta[i], tb[i]):
			level = NameNear
			near++
		default:
			return NameNone
		}
	}
	if near > max(1, len(ta)/2) {
		return NameNone
	}
	return level
}

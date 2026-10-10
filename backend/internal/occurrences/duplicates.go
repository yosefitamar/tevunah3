package occurrences

import (
	"strings"
	"time"
	"unicode"

	"github.com/belia/tevunah/backend/internal/intel"
)

// NormCIOPS devolve a forma de comparação da ficha CIOPS: só letras e
// dígitos, em maiúsculas. Espelha app.norm_ciops() — espaço, hífen, ponto e
// caixa são variações de digitação, não fichas diferentes.
func NormCIOPS(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// Motivos pelos quais duas ocorrências com fichas diferentes parecem ser a
// mesma. A tela traduz os códigos.
const (
	ReasonSameTime         = "same_time"         // mesma data, hora a até 30 min
	ReasonSameCity         = "same_city"         // mesmo município
	ReasonSameNeighborhood = "same_neighborhood" // mesmo bairro
	ReasonSimilarCIOPS     = "similar_ciops"     // fichas a até 2 caracteres de distância
)

// Limites da comparação. Meia hora cobre a diferença entre a hora do fato e
// a do acionamento; duas edições cobrem dígito trocado, faltando ou sobrando
// e a inversão de dois vizinhos.
const (
	timeWindow       = 30 * time.Minute
	ciopsMaxDistance = 2
	ciopsMinLength   = 6
)

// Subject é o que se sabe da ocorrência que está entrando (ou sendo
// comparada): o bastante para dizer se já existe outra igual.
type Subject struct {
	CIOPS        string
	OccurredOn   time.Time
	Time         string // "HH:MM"; "" = hora desconhecida
	City         string
	Neighborhood string
}

// matchReasons compara duas ocorrências de fichas diferentes e devolve por
// que parecem a mesma — nil quando não parecem.
//
// A regra não trata ficha igual: isso é identidade (mesma ocorrência), não
// suspeita, e é resolvido antes. Aqui ficam os casos em que a ficha não
// denuncia a repetição — digitada errado de um lado, ou ausente:
//
//   - mesma hora (±30 min) no mesmo município;
//   - mesmo bairro no mesmo dia, quando um dos lados não tem hora;
//   - fichas quase iguais no mesmo dia, sem município divergente.
//
// Município divergente descarta: vem de lista fechada, não é erro de
// digitação. Bairro divergente não descarta — é texto livre, e a grafia
// varia entre o relatório e o cadastro.
func matchReasons(a, b Subject) []string {
	ka, kb := NormCIOPS(a.CIOPS), NormCIOPS(b.CIOPS)
	if ka != "" && ka == kb {
		return nil
	}
	cityA, cityB := intel.Normalize(a.City), intel.Normalize(b.City)
	if cityA != "" && cityB != "" && cityA != cityB {
		return nil
	}
	sameCity := cityA != "" && cityA == cityB
	neighA, neighB := intel.Normalize(a.Neighborhood), intel.Normalize(b.Neighborhood)
	sameNeigh := neighA != "" && neighA == neighB

	sameDay := sameDate(a.OccurredOn, b.OccurredOn)
	ta, okA := clock(a.OccurredOn, a.Time)
	tb, okB := clock(b.OccurredOn, b.Time)
	timeClose := okA && okB && absDuration(ta.Sub(tb)) <= timeWindow

	similar := len(ka) >= ciopsMinLength && len(kb) >= ciopsMinLength &&
		editDistance(ka, kb, ciopsMaxDistance) <= ciopsMaxDistance

	switch {
	case timeClose && sameCity:
	case sameDay && sameCity && sameNeigh && (!okA || !okB):
	case similar && (sameDay || timeClose):
	default:
		return nil
	}

	var reasons []string
	if similar {
		reasons = append(reasons, ReasonSimilarCIOPS)
	}
	if timeClose {
		reasons = append(reasons, ReasonSameTime)
	}
	if sameNeigh {
		reasons = append(reasons, ReasonSameNeighborhood)
	} else if sameCity {
		reasons = append(reasons, ReasonSameCity)
	}
	return reasons
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// clock junta a data com "HH:MM". Compara instantes e não só relógios, para
// 23:50 e 00:10 do dia seguinte ficarem a 20 minutos, não a quase um dia.
func clock(day time.Time, hhmm string) (time.Time, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		return time.Time{}, false
	}
	y, m, d := day.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, time.UTC), true
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// editDistance é a distância de Damerau-Levenshtein (com transposição de
// vizinhos — "0705888" × "0750888" conta 1). Para cedo: diferença de tamanho
// acima de max já responde max+1.
func editDistance(a, b string, max int) int {
	if d := len(a) - len(b); d > max || -d > max {
		return max + 1
	}
	prev2 := make([]int, len(b)+1)
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			v := min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				v = min(v, prev2[j-2]+1)
			}
			cur[j] = v
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(b)]
}

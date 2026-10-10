// Package intel identifica a participação da inteligência (SAI) nas
// ocorrências a partir de termos configuráveis, e guarda esses termos.
package intel

import (
	"strings"
	"unicode"
)

// Normalize reduz um texto a palavras separadas por um espaço: sem acento,
// sem caixa, sem pontuação, e com letra e dígito colados separados. É o que
// torna a comparação tolerante aos erros de digitação mais comuns da ficha:
//
//	"SAI do 2º BPRAIO"   → "sai do 2 bpraio"
//	"SAI/2ºBPRAIO"       → "sai 2 bpraio"
//	"S.A.I."             → "s a i"   (não confunde com o verbo "sai")
//	"P-2", "P/2", "P2"   → "p 2"
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prev := ' ' // classe do último caractere escrito: ' ', 'a' (letra) ou '0' (dígito)
	for _, r := range s {
		r = unicode.ToLower(r)
		if f, ok := diacritics[r]; ok {
			r = f
		}
		var class rune
		switch {
		case r == 'º' || r == 'ª' || r == '°':
			// Ordinal colado ao número ("2º"): some sem quebrar a palavra.
			continue
		case unicode.IsLetter(r):
			class = 'a'
		case unicode.IsDigit(r):
			class = '0'
		default:
			class = ' '
		}
		if class == ' ' {
			if prev != ' ' {
				b.WriteByte(' ')
			}
			prev = ' '
			continue
		}
		if prev != ' ' && prev != class {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		prev = class
	}
	return strings.TrimSpace(b.String())
}

var diacritics = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y',
}

// Matcher compara textos contra um conjunto de termos já normalizados.
type Matcher struct {
	terms []matcherTerm
}

type matcherTerm struct {
	label string // como o administrador cadastrou
	norm  string // " sai do 2 bpraio " — com espaços nas pontas
}

// NewMatcher prepara os termos. Termos que normalizam para vazio (só
// pontuação) são ignorados.
func NewMatcher(terms []string) *Matcher {
	m := &Matcher{}
	for _, t := range terms {
		n := Normalize(t)
		if n == "" {
			continue
		}
		m.terms = append(m.terms, matcherTerm{label: strings.TrimSpace(t), norm: " " + n + " "})
	}
	return m
}

// Match devolve os termos encontrados em qualquer dos textos, na ordem de
// cadastro e sem repetição. O termo precisa aparecer como palavra(s)
// inteira(s): "SAI do 2º BPRAIO" não casa com "saiu do 2º BPRAIO".
func (m *Matcher) Match(texts ...string) []string {
	if m == nil || len(m.terms) == 0 {
		return nil
	}
	parts := make([]string, 0, len(texts))
	for _, t := range texts {
		if n := Normalize(t); n != "" {
			parts = append(parts, n)
		}
	}
	// Os textos são unidos por " | " para que um termo nunca case juntando o
	// fim de um campo com o começo de outro.
	hay := " " + strings.Join(parts, " | ") + " "
	var out []string
	for _, t := range m.terms {
		if strings.Contains(hay, t.norm) {
			out = append(out, t.label)
		}
	}
	return out
}

package opsreport

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// LayoutText devolve o texto das páginas first..last do PDF como o poppler o
// diagrama (`pdftotext -layout`): rótulos e valores na posição em que estão
// impressos. É o "relatório original" que a IA relê para corrigir o que a
// importação leu errado.
func LayoutText(ctx context.Context, path string, first, last int) (string, error) {
	if first < 1 {
		first = 1
	}
	if last < first {
		last = first
	}
	ctx, cancel := context.WithTimeout(ctx, extractTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8",
		"-f", strconv.Itoa(first), "-l", strconv.Itoa(last), path, "-")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w (%s)", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return stdout.String(), nil
}

var (
	reFichaNum   = regexp.MustCompile(`M\d{11}`)
	reNatureLine = regexp.MustCompile(`^\s*Natureza\s*:`)
	// Cabeçalho e rodapé de página impressos pelo navegador, e o título do
	// relatório: não são da ocorrência.
	rePageNoise = []*regexp.Regexp{
		regexp.MustCompile(`^\s*https?://\S+\s+\d+/\d+\s*$`),
		regexp.MustCompile(`^\s*\d{2}/\d{2}/\d{2},\s*\d{2}:\d{2}\s+Relatório de Ocorrências\s*$`),
		regexp.MustCompile(`^\s*Comando de Policiamento`),
		regexp.MustCompile(`^\s*RELATÓRIO DIÁRIO DE OCORRÊNCIAS\s*$`),
		regexp.MustCompile(`^\s*log_\w+\s*$`),
		regexp.MustCompile(`^\s*\d+º\s*BPRAIO\s*$`),
		regexp.MustCompile(`^\s*Data:\s*\d{2}/\d{2}/\d{4}\s*$`),
	}
)

// maxSourceChars limita o trecho mandado à IA (uma ocorrência longa ocupa
// duas páginas; mais que isso é recorte errado).
const maxSourceChars = 16000

// OccurrenceSource recorta do texto das páginas o bloco da ocorrência da
// ficha: da linha "Natureza:" que abre o bloco até a que abre o seguinte.
// A ficha é procurada pelo número (M + 11 dígitos), mesmo que a gravada
// tenha lixo grudado. Sem achar a ficha, devolve "" — recortar a ocorrência
// errada seria pior que não reler.
func OccurrenceSource(text, ciops string) string {
	num := reFichaNum.FindString(strings.ToUpper(ciops))
	if num == "" {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(text, "\f", "\n"), "\n")
	at := -1
	for i, l := range lines {
		if strings.Contains(l, num) {
			at = i
			break
		}
	}
	if at < 0 {
		return ""
	}
	start := at
	for i := at; i >= 0 && i >= at-4; i-- {
		if reNatureLine.MatchString(lines[i]) {
			start = i
			break
		}
	}
	end := len(lines)
	for i := at + 1; i < len(lines); i++ {
		if reNatureLine.MatchString(lines[i]) {
			end = i
			break
		}
	}
	var out []string
	blank := 0
	for _, l := range lines[start:end] {
		l = strings.TrimRight(l, " \t\r")
		if noise(l) {
			continue
		}
		if strings.TrimSpace(l) == "" {
			if blank++; blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	s := strings.TrimSpace(dedent(out))
	if r := []rune(s); len(r) > maxSourceChars {
		s = string(r[:maxSourceChars])
	}
	return s
}

func noise(l string) bool {
	for _, re := range rePageNoise {
		if re.MatchString(l) {
			return true
		}
	}
	return false
}

// dedent tira a margem comum (o layout recua tudo uns 9 espaços).
func dedent(lines []string) string {
	min := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		if min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return strings.Join(lines, "\n")
	}
	for i, l := range lines {
		if len(l) >= min {
			lines[i] = l[min:]
		}
	}
	return strings.Join(lines, "\n")
}

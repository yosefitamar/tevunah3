package opsreport

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Word é uma palavra do PDF com a caixa que ocupa na página (pontos, origem
// no canto superior esquerdo). A posição é o que permite ler as tabelas: a
// célula vazia não deixa rastro no texto corrido, só no x das vizinhas.
type Word struct {
	X0, Y0, X1, Y1 float64
	Text           string
}

// Page é a lista de palavras de uma página, na ordem em que o poppler as
// emitiu.
type Page struct {
	Words []Word
}

// ErrNoText indica PDF sem camada de texto (digitalizado). O relatório
// oficial é gerado pelo navegador e sempre tem texto — escaneado exigiria OCR,
// que está fora do escopo.
var ErrNoText = errors.New("o PDF não tem texto extraível (é uma imagem digitalizada?)")

// extractTimeout limita o pdftotext. O relatório diário (30–40 páginas) sai
// em menos de um segundo; o teto só protege contra PDF malformado.
const extractTimeout = 20 * time.Second

// Extract roda o pdftotext (poppler-utils) no modo -bbox e devolve as
// palavras de cada página. O PDF passa por arquivo temporário porque o
// pdftotext precisa de acesso aleatório ao arquivo.
func Extract(ctx context.Context, pdf []byte) ([]Page, error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("arquivo não é PDF")
	}
	tmp, err := os.CreateTemp("", "opsreport-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(pdf); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("temp: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, extractTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-bbox", "-enc", "UTF-8", tmp.Name(), "-")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext: %w (%s)", err, bytes.TrimSpace(stderr.Bytes()))
	}
	pages, err := ParseBBox(&stdout)
	if err != nil {
		return nil, err
	}
	for _, p := range pages {
		if len(p.Words) > 0 {
			return pages, nil
		}
	}
	return nil, ErrNoText
}

// ParseBBox lê o XHTML do `pdftotext -bbox`: <page> com <word xMin yMin xMax
// yMax>texto</word>.
func ParseBBox(r io.Reader) ([]Page, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	var pages []Page
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("bbox: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "page":
			pages = append(pages, Page{})
		case "word":
			if len(pages) == 0 {
				continue
			}
			var w struct {
				Text string `xml:",chardata"`
			}
			if err := dec.DecodeElement(&w, &se); err != nil {
				return nil, fmt.Errorf("bbox word: %w", err)
			}
			word := Word{Text: w.Text}
			for _, a := range se.Attr {
				v, _ := strconv.ParseFloat(a.Value, 64)
				switch a.Name.Local {
				case "xMin":
					word.X0 = v
				case "yMin":
					word.Y0 = v
				case "xMax":
					word.X1 = v
				case "yMax":
					word.Y1 = v
				}
			}
			if word.Text != "" {
				pages[len(pages)-1].Words = append(pages[len(pages)-1].Words, word)
			}
		}
	}
	return pages, nil
}

package opsreport

import (
	"strings"
	"testing"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// As fotos cabem no envio: a de mais de 5 MB é reduzida, as pequenas vão
// como estão; sem como reduzir, é erro (e não um 413 do SIPOM).
func TestFitPhotos(t *testing.T) {
	var got []string
	mk := func(n int) payloadPhoto {
		return payloadPhoto{data: make([]byte, n), mime: "image/png", set: func(b64, mime string) { got = append(got, mime) }}
	}
	shrunk := 0
	shrink := func([]byte) ([]byte, error) { shrunk++; return make([]byte, 100), nil }
	if err := fitPhotos([]payloadPhoto{mk(10), mk(sipom.MaxFotoBytes + 1)}, shrink); err != nil {
		t.Fatal(err)
	}
	if shrunk != 1 || strings.Join(got, ",") != "image/png,image/jpeg" {
		t.Errorf("reduzidas %d, tipos %v", shrunk, got)
	}
	// Juntas passam do orçamento do corpo: reduz até caber.
	got, shrunk = nil, 0
	list := []payloadPhoto{mk(4 << 20), mk(4 << 20), mk(4 << 20), mk(4 << 20)}
	if err := fitPhotos(list, shrink); err != nil {
		t.Fatal(err)
	}
	if shrunk == 0 || shrunk == len(list) {
		t.Errorf("deveria reduzir só até caber: %d de %d", shrunk, len(list))
	}
	if err := fitPhotos([]payloadPhoto{mk(sipom.MaxFotoBytes + 1)}, nil); err == nil {
		t.Errorf("sem Shrink, foto grande deveria dar erro")
	}
}

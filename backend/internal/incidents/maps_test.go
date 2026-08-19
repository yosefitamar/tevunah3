package incidents

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestResolveMapsLink_HostNaoPermitido(t *testing.T) {
	// A allowlist é o que separa "resolver um link do Maps" de "buscar
	// qualquer URL que o usuário colar, de dentro da rede da agência".
	for _, raw := range []string{
		"https://evil.example.com/maps/@-3.85,-40.92,17z",
		"http://maps.app.goo.gl/abc",       // sem TLS
		"https://maps.app.goo.gl.evil.com/x", // sufixo forjado
		"file:///etc/passwd",
		"https://169.254.169.254/latest/meta-data/",
		"",
	} {
		if _, err := ResolveMapsLink(context.Background(), raw); !errors.Is(err, ErrMapsHost) {
			t.Errorf("ResolveMapsLink(%q) = %v, quero ErrMapsHost", raw, err)
		}
	}
}

func TestResolveMapsLink_CoordenadaNaPropriaURL(t *testing.T) {
	// URL longa já traz o par: resolve sem tocar na rede.
	p, err := ResolveMapsLink(context.Background(),
		"https://www.google.com/maps/place/Ubajara/@-3.8541234,-40.9212345,17z/")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if !p.HasCoords || p.Latitude != -3.8541234 || p.Longitude != -40.9212345 {
		t.Errorf("ponto = %+v", p)
	}
}

func TestResolveMapsLink_Desligado(t *testing.T) {
	t.Setenv("INCIDENT_MAPS_RESOLVER", "off")
	if MapsResolverEnabled() {
		t.Fatal("MapsResolverEnabled = true com INCIDENT_MAPS_RESOLVER=off")
	}
	_, err := ResolveMapsLink(context.Background(), "https://maps.app.goo.gl/abc")
	if !errors.Is(err, ErrMapsDisabled) {
		t.Errorf("err = %v, quero ErrMapsDisabled", err)
	}
}

// TestResolveMapsLink_Live bate no Google de verdade. Roda só com
// TEVUNAH_MAPS_LIVE=1 — teste de rede não pode quebrar a suíte.
func TestResolveMapsLink_Live(t *testing.T) {
	if os.Getenv("TEVUNAH_MAPS_LIVE") != "1" {
		t.Skip("TEVUNAH_MAPS_LIVE != 1")
	}
	p, err := ResolveMapsLink(context.Background(),
		"https://maps.app.goo.gl/TDUFg2LSqbvamRkm9?g_st=ic")
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	t.Logf("resolvido: %+v", p)
	if !p.HasCoords {
		t.Fatal("link sem coordenadas")
	}
	if p.Latitude > -2 || p.Latitude < -8 || p.Longitude > -37 || p.Longitude < -42 {
		t.Errorf("coordenada fora do Ceará: %v/%v", p.Latitude, p.Longitude)
	}
}

package geocode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
)

// fakeNominatim responde como o /search do Nominatim e anota as consultas.
type fakeNominatim struct {
	streets map[string][3]any // "número rua|cidade" → lat, lon, place_rank
	places  map[string][3]any // q → lat, lon, place_rank
	calls   []string
}

func (f *fakeNominatim) handler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.calls = append(f.calls, r.URL.RawQuery)
	if q.Get("format") != "jsonv2" || q.Get("countrycodes") != "br" || q.Get("limit") != "1" {
		http.Error(w, "parâmetros", http.StatusBadRequest)
		return
	}
	var hit [3]any
	var ok bool
	if s := q.Get("street"); s != "" {
		hit, ok = f.streets[s+"|"+q.Get("city")]
	} else {
		if q.Get("featureType") != "settlement" {
			http.Error(w, "featureType", http.StatusBadRequest)
			return
		}
		hit, ok = f.places[q.Get("q")]
	}
	out := []map[string]any{}
	if ok {
		out = append(out, map[string]any{"lat": hit[0], "lon": hit[1], "place_rank": hit[2]})
	}
	_ = json.NewEncoder(w).Encode(out)
}

func TestLocate(t *testing.T) {
	fake := &fakeNominatim{
		streets: map[string][3]any{
			"317 Rua Quintino Cunha|CAUCAIA":  {"-3.7361", "-38.6531", 30}, // número achado
			"999 Rua Sem Numero|CAUCAIA":      {"-3.7400", "-38.6600", 26}, // só a rua
			"Rua Fabiano De Cristo|CAUCAIA":   {"-3.7500", "-38.6700", 26},
			"10 Rua Que So Acha Cidade|CRATO": {"-7.2300", "-39.4100", 16}, // caiu na cidade: não serve
		},
		places: map[string][3]any{
			"SÃO MIGUEL, CAUCAIA, Ceará": {"-3.7300", "-38.6500", 20},
			"CENTRO, CRATO, Ceará":       {"-7.2340", "-39.4090", 16}, // devolveu a cidade: não serve
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	g := New(srv.URL+"/", "Ceará", nil)
	ctx := context.Background()

	for _, c := range []struct {
		name string
		q    Query
		want *Result
	}{
		{"número localizado",
			Query{Street: "Rua Quintino Cunha", Number: "317", Neighborhood: "SÃO MIGUEL", City: "CAUCAIA"},
			&Result{-3.7361, -38.6531, PrecisionDoor}},
		{"complemento junto do número é descartado",
			Query{Street: " Rua  Quintino Cunha ", Number: "317 - Casa 39a", City: "CAUCAIA"},
			&Result{-3.7361, -38.6531, PrecisionDoor}},
		{"só a rua",
			Query{Street: "Rua Sem Numero", Number: "999", City: "CAUCAIA"},
			&Result{-3.7400, -38.6600, PrecisionStreet}},
		{"S/N busca só a rua",
			Query{Street: "Rua Fabiano De Cristo", Number: "S/N", City: "CAUCAIA"},
			&Result{-3.7500, -38.6700, PrecisionStreet}},
		{"rua não achada: cai no bairro, aproximado",
			Query{Street: "Rua Inexistente", Number: "5", Neighborhood: "SÃO MIGUEL", City: "CAUCAIA"},
			&Result{-3.7300, -38.6500, PrecisionNeighborhood}},
		{"resultado no nível da cidade não é coordenada",
			Query{Street: "Rua Que So Acha Cidade", Number: "10", Neighborhood: "CENTRO", City: "CRATO"},
			nil},
		{"sem cidade não consulta", Query{Street: "Rua Quintino Cunha", Number: "317"}, nil},
		{"só a cidade não consulta", Query{City: "CAUCAIA"}, nil},
	} {
		got, err := g.Locate(ctx, c.q)
		if err != nil {
			t.Errorf("%s: erro %v", c.name, err)
			continue
		}
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: %+v, quer %+v", c.name, got, c.want)
		}
	}
}

func TestDisabledAndFailure(t *testing.T) {
	if g := New("  ", "Ceará", nil); g != nil || g.Enabled() {
		t.Fatalf("sem URL o geocodificador deve ficar desligado")
	}
	// Desligado: Locate não consulta nem falha.
	var off *Geocoder
	if res, err := off.Locate(context.Background(), Query{Street: "Rua A", City: "CAUCAIA"}); res != nil || err != nil {
		t.Errorf("desligado: %+v, %v", res, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "fora do ar", http.StatusBadGateway)
	}))
	defer srv.Close()
	// Serviço fora do ar é erro — diferente de endereço não localizado.
	if res, err := New(srv.URL, "", nil).Locate(context.Background(), Query{Street: "Rua A", City: "CAUCAIA"}); err == nil || res != nil {
		t.Errorf("serviço fora do ar: %+v, %v", res, err)
	}
}

// O cache poupa o serviço: o mesmo endereço (em qualquer grafia) só é
// consultado uma vez, inclusive quando a resposta foi "não localizado".
// Contra um Postgres real; pula sem APP_DATABASE_URL.
func TestSmoke_Cache(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// O papel da aplicação não apaga; a conexão das migrations limpa as
	// entradas de teste (antes, para o run ser repetível, e no fim).
	cleanup := func() {
		if su := os.Getenv("MIGRATIONS_DATABASE_URL"); su != "" {
			if c, err := idb.Open(su); err == nil {
				defer c.Close()
				_, _ = c.ExecContext(ctx, `DELETE FROM app.geocode_cache WHERE query_key LIKE '%|smoke city do cache|%'`)
			}
		}
	}
	cleanup()
	defer cleanup()

	fake := &fakeNominatim{
		streets: map[string][3]any{"12 Rua do Cache|SMOKE CITY DO CACHE": {"-3.1000", "-38.2000", 30}},
	}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	defer srv.Close()
	g := New(srv.URL, "Ceará", db)

	found := Query{Street: "Rua do Cache", Number: "12", City: "SMOKE CITY DO CACHE"}
	for i, q := range []Query{found, {Street: "rua  do cache", Number: "12 - fundos", City: "Smoke City do Cache"}} {
		res, err := g.Locate(ctx, q)
		if err != nil || res == nil || res.Lat != -3.1 || res.Lng != -38.2 || res.Precision != PrecisionDoor {
			t.Fatalf("consulta %d: %+v, %v", i, res, err)
		}
	}
	if len(fake.calls) != 1 {
		t.Errorf("endereço localizado consultou o serviço %d vezes; quero 1", len(fake.calls))
	}

	// Não localizado: rua + bairro = 2 consultas na primeira vez, nenhuma na segunda.
	missing := Query{Street: "Rua Que Nao Existe", Neighborhood: "Bairro Fantasma", City: "SMOKE CITY DO CACHE"}
	before := len(fake.calls)
	for i := 0; i < 2; i++ {
		if res, err := g.Locate(ctx, missing); err != nil || res != nil {
			t.Fatalf("não localizado (%d): %+v, %v", i, res, err)
		}
	}
	if got := len(fake.calls) - before; got != 2 {
		t.Errorf("não localizado consultou o serviço %d vezes; quero 2 (rua e bairro, só na primeira)", got)
	}
}

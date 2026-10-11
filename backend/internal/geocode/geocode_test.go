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
	streets map[string][3]any     // "número rua|cidade" → lat, lon, place_rank
	places  map[string][3]any     // q com featureType=settlement → lat, lon, place_rank
	cities  map[string][3]any     // cidade (busca só da sede) → lat, lon, place_rank
	free    map[string][]fakeItem // q em texto livre → lugares
	calls   []string
}

type fakeItem struct {
	lat, lon            string
	rank                int
	name, category, typ string
	city                string
}

func (f *fakeNominatim) handler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.calls = append(f.calls, r.URL.RawQuery)
	if q.Get("format") != "jsonv2" || q.Get("countrycodes") != "br" || q.Get("limit") == "" {
		http.Error(w, "parâmetros", http.StatusBadRequest)
		return
	}
	out := []map[string]any{}
	add := func(hit [3]any, ok bool) {
		if ok {
			out = append(out, map[string]any{"lat": hit[0], "lon": hit[1], "place_rank": hit[2],
				"category": "highway", "display_name": "x, " + q.Get("city") + ", Ceará"})
		}
	}
	switch {
	case q.Get("street") != "":
		hit, ok := f.streets[q.Get("street")+"|"+q.Get("city")]
		add(hit, ok)
	case q.Get("q") == "":
		hit, ok := f.cities[q.Get("city")]
		add(hit, ok)
	case q.Get("featureType") == "settlement":
		hit, ok := f.places[q.Get("q")]
		add(hit, ok)
	default:
		for _, it := range f.free[q.Get("q")] {
			out = append(out, map[string]any{"lat": it.lat, "lon": it.lon, "place_rank": it.rank,
				"name": it.name, "category": it.category, "type": it.typ,
				"display_name": it.name + ", " + it.city + ", Ceará, Brasil"})
		}
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
			"Rua 47|MARACANAÚ":                {"-3.8804", "-38.6136", 26},
		},
		places: map[string][3]any{
			"SÃO MIGUEL, CAUCAIA, Ceará": {"-3.7300", "-38.6500", 20},
			"CENTRO, CRATO, Ceará":       {"-7.2340", "-39.4090", 16}, // devolveu a cidade: não serve
		},
		cities: map[string][3]any{
			"TRAIRI":     {"-3.2777", "-39.2680", 16},
			"MARANGUAPE": {"-3.8923", "-38.6835", 16},
			"HORIZONTE":  {"-4.0940", "-38.4626", 16},
		},
		free: map[string][]fakeItem{
			// Sem "rodrigres" a busca traz as ruas do Francisco.
			"francisco, HORIZONTE, Ceará": {
				{"-4.1000", "-38.4800", 26, "Rua Francisco Alves", "highway", "residential", "Horizonte"},
				{"-4.1073", "-38.4839", 26, "Rua Francisco Rodrigues", "highway", "residential", "Horizonte"},
			},
			"TABATINGA, MARANGUAPE, Ceará": {
				{"-3.9000", "-38.7000", 30, "Mercadinho Tabatinga", "shop", "supermarket", "Maranguape"},
				{"-3.9576", "-38.6972", 24, "Cemiterio da Tabatinga", "landuse", "cemetery", "Maranguape"},
			},
			"OUTRA BANDA, MARANGUAPE, Ceará": {
				{"-3.9100", "-38.7100", 30, "Bar Outra Banda", "amenity", "bar", "Maranguape"},
				{"-3.1000", "-39.0000", 24, "Escola Outra Banda", "amenity", "school", "Itapipoca"}, // outro município
			},
			"CE-065, MARANGUAPE, Ceará": {
				{"-3.8863", "-38.6777", 26, "CE-065", "highway", "primary", "Maranguape"},
			},
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
		{"esquina: vale o trecho que o mapa tem",
			Query{Street: "Av. 12 com Rua 47", Neighborhood: "JEREISSATI I", City: "MARACANAÚ"},
			&Result{-3.8804, -38.6136, PrecisionStreet}},
		{"rua com erro de digitação",
			Query{Street: "RUA FRANCISCO RODRIGRES", Number: "190", Neighborhood: "PLANALTO", City: "HORIZONTE"},
			&Result{-4.1073, -38.4839, PrecisionStreet}},
		{"rua não achada: cai no bairro, aproximado",
			Query{Street: "Rua Inexistente", Number: "5", Neighborhood: "SÃO MIGUEL", City: "CAUCAIA"},
			&Result{-3.7300, -38.6500, PrecisionNeighborhood}},
		{"bairro fora do mapa: lugar público com o nome dele, não o comércio",
			Query{Street: "Avenida Tabatinga", Number: "2141", Neighborhood: "TABATINGA", City: "MARANGUAPE"},
			&Result{-3.9576, -38.6972, PrecisionNeighborhood}},
		{"rodovia citada vem depois do bairro",
			Query{Street: "CE 065, Avenida Tabatinga", Number: "202", Neighborhood: "TABATINGA", City: "MARANGUAPE"},
			&Result{-3.9576, -38.6972, PrecisionNeighborhood}},
		{"rodovia citada, sem bairro no mapa",
			Query{Street: "CE 065, Km 12", Neighborhood: "SEPUPARÁ", City: "MARANGUAPE"},
			&Result{-3.8863, -38.6777, PrecisionStreet}},
		{"só comércio e lugar de outro município: sede",
			Query{Street: "Rua Hebrom", Neighborhood: "OUTRA BANDA", City: "MARANGUAPE"},
			&Result{-3.8923, -38.6835, PrecisionCity}},
		{"zona rural sem rua no mapa: sede do município",
			Query{Street: "Córrego Da Ramada", Neighborhood: "ZONA RURAL", City: "TRAIRI"},
			&Result{-3.2777, -39.2680, PrecisionCity}},
		{"cidade que o mapa não acha: não localizado",
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

	// Não localizado: desce todos os degraus na primeira vez, nenhum na segunda.
	missing := Query{Street: "Rua Que Nao Existe", Neighborhood: "Bairro Fantasma", City: "SMOKE CITY DO CACHE"}
	var calls [2]int
	for i := range calls {
		before := len(fake.calls)
		if res, err := g.Locate(ctx, missing); err != nil || res != nil {
			t.Fatalf("não localizado (%d): %+v, %v", i, res, err)
		}
		calls[i] = len(fake.calls) - before
	}
	if calls[0] == 0 || calls[1] != 0 {
		t.Errorf("não localizado consultou o serviço %d e depois %d vezes; quero consultas só na primeira", calls[0], calls[1])
	}
}

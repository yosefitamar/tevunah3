// Package geocode converte o endereço de uma ocorrência em coordenada,
// consultando um Nominatim (OpenStreetMap) — o da própria agência
// (GEOCODER_URL).
//
// O serviço é da casa de propósito: não tem custo por consulta nem conta em
// provedor externo, e endereço de ocorrência não sai da rede. Sem
// GEOCODER_URL o geocodificador fica desligado (nil) e o sistema segue como
// antes — coordenada só a que o analista informar.
package geocode

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/intel"
)

// Precisão do ponto encontrado.
const (
	PrecisionDoor         = "porta"  // número localizado na rua
	PrecisionStreet       = "rua"    // rua localizada, número não
	PrecisionNeighborhood = "bairro" // só o bairro: ponto aproximado
)

// Faixas de place_rank do Nominatim: 26–27 é via; 28 em diante, endereço ou
// ponto na via; 17–25 vai de distrito a bairro/loteamento. Até 16 é a cidade
// (ou maior) — largo demais para ser a coordenada de uma ocorrência.
const (
	rankStreet    = 26
	rankDoor      = 28
	rankCityLimit = 16
)

// negativeTTL: por quanto tempo um "não localizado" fica no cache. O mapa da
// agência é atualizado de tempos em tempos, e a rua que faltava pode chegar.
const negativeTTL = 7 * 24 * time.Hour

// Query é o endereço a localizar, como a ocorrência o tem.
type Query struct {
	Street       string // logradouro, sem o número
	Number       string // número; "S/N" ou vazio = sem número
	Neighborhood string
	City         string
}

// Result é o ponto encontrado.
type Result struct {
	Lat, Lng  float64
	Precision string
}

// provider é quem de fato procura o endereço.
type provider interface {
	// name identifica o provedor (entra na chave do cache).
	name() string
	// lookup devolve (nil, nil) quando o endereço não foi localizado; erro
	// só para falha do serviço.
	lookup(ctx context.Context, q Query) (*Result, error)
}

// Geocoder consulta o provedor e guarda as respostas em app.geocode_cache.
type Geocoder struct {
	p     provider
	state string
	db    *sql.DB // nil = sem cache
}

// New devolve o geocodificador sobre um Nominatim, ou nil quando baseURL está
// vazio (desligado). state restringe a busca à UF ("Ceará") — evita a rua
// homônima de outro estado.
func New(baseURL, state string, db *sql.DB) *Geocoder {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	state = strings.TrimSpace(state)
	return &Geocoder{
		p:     &nominatim{base: baseURL, state: state, http: newHTTPClient()},
		state: state, db: db,
	}
}

// FromEnv monta o geocodificador da configuração do ambiente:
//
//	GEOCODER_URL    endereço do Nominatim; vazio = desligado
//	GEOCODER_STATE  UF em que os endereços são procurados (padrão Ceará)
//
// O segundo retorno descreve o estado, para o log da subida.
func FromEnv(db *sql.DB) (*Geocoder, string) {
	base := strings.TrimSpace(os.Getenv("GEOCODER_URL"))
	state := strings.TrimSpace(os.Getenv("GEOCODER_STATE"))
	if state == "" {
		state = "Ceará"
	}
	if base == "" {
		return nil, "desligado (GEOCODER_URL vazio): coordenada só a informada pelo analista"
	}
	return New(base, state, db), "ligado: Nominatim em " + base
}

func newHTTPClient() *http.Client { return &http.Client{Timeout: 6 * time.Second} }

// Enabled diz se há geocodificador configurado.
func (g *Geocoder) Enabled() bool { return g != nil }

// Locate procura o endereço. Tenta a rua (com o número, se houver) dentro da
// cidade; sem resultado, o bairro — ponto aproximado. Devolve (nil, nil)
// quando nada foi localizado; erro só para falha do serviço.
func (g *Geocoder) Locate(ctx context.Context, q Query) (*Result, error) {
	if g == nil {
		return nil, nil
	}
	q = q.clean()
	if q.City == "" || (q.Street == "" && q.Neighborhood == "") {
		return nil, nil
	}
	key := g.cacheKey(q)
	if res, hit := g.cached(ctx, key); hit {
		return res, nil
	}

	res, err := g.p.lookup(ctx, q)
	if err != nil {
		return nil, err
	}
	g.store(ctx, key, res)
	return res, nil
}

// ─── Nominatim ───

// nominatim consulta um Nominatim (OpenStreetMap) — o da própria agência.
type nominatim struct {
	base  string
	state string
	http  *http.Client
}

func (g *nominatim) name() string { return "nominatim" }

func (g *nominatim) lookup(ctx context.Context, q Query) (*Result, error) {
	if q.Street != "" {
		street := q.Street
		if q.Number != "" {
			street = q.Number + " " + q.Street
		}
		p := url.Values{"street": {street}, "city": {q.City}}
		if g.state != "" {
			p.Set("state", g.state)
		}
		hit, err := g.search(ctx, p)
		if err != nil {
			return nil, err
		}
		if hit != nil && hit.rank >= rankStreet {
			prec := PrecisionStreet
			if q.Number != "" && hit.rank >= rankDoor {
				prec = PrecisionDoor
			}
			return &Result{Lat: hit.lat, Lng: hit.lng, Precision: prec}, nil
		}
	}
	if q.Neighborhood != "" {
		parts := []string{q.Neighborhood, q.City}
		if g.state != "" {
			parts = append(parts, g.state)
		}
		// featureType=settlement: só localidade (bairro, distrito, povoado),
		// nunca uma rua ou comércio que por acaso tenha o nome do bairro.
		hit, err := g.search(ctx, url.Values{"q": {strings.Join(parts, ", ")}, "featureType": {"settlement"}})
		if err != nil {
			return nil, err
		}
		if hit != nil && hit.rank > rankCityLimit && hit.rank < rankStreet {
			return &Result{Lat: hit.lat, Lng: hit.lng, Precision: PrecisionNeighborhood}, nil
		}
	}
	return nil, nil
}

type hit struct {
	lat, lng float64
	rank     int
}

func (g *nominatim) search(ctx context.Context, p url.Values) (*hit, error) {
	p.Set("format", "jsonv2")
	p.Set("limit", "1")
	p.Set("countrycodes", "br")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+"/search?"+p.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tevunah-geocode")
	resp, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("geocodificador: %w", withoutURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocodificador: HTTP %d", resp.StatusCode)
	}
	var items []struct {
		Lat       string `json:"lat"`
		Lon       string `json:"lon"`
		PlaceRank int    `json:"place_rank"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&items); err != nil {
		return nil, fmt.Errorf("geocodificador: resposta inválida: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	lat, err1 := strconv.ParseFloat(items[0].Lat, 64)
	lng, err2 := strconv.ParseFloat(items[0].Lon, 64)
	if err1 != nil || err2 != nil {
		return nil, errors.New("geocodificador: coordenada inválida na resposta")
	}
	return &hit{lat: lat, lng: lng, rank: items[0].PlaceRank}, nil
}

// ─── Entrada ───

var leadingNumber = regexp.MustCompile(`^\d+`)

// clean tira o que atrapalha a busca: espaços repetidos e o complemento que
// acompanha o número ("335 - Casa 39a" → "335"; "S/N" → sem número).
func (q Query) clean() Query {
	sp := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	return Query{
		Street:       sp(q.Street),
		Number:       leadingNumber.FindString(strings.TrimSpace(q.Number)),
		Neighborhood: sp(q.Neighborhood),
		City:         sp(q.City),
	}
}

// ─── Cache ───

func (g *Geocoder) cacheKey(q Query) string {
	parts := []string{q.Street, q.Number, q.Neighborhood, q.City, g.state}
	for i := range parts {
		parts[i] = intel.Normalize(parts[i])
	}
	return g.p.name() + "|" + strings.Join(parts, "|")
}

// withoutURL tira a URL do erro de rede: ela leva o endereço consultado, que
// não deve ir para o log.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// cached devolve a resposta guardada. hit=false manda consultar: não há
// registro, ou o "não localizado" guardado já venceu.
func (g *Geocoder) cached(ctx context.Context, key string) (res *Result, hit bool) {
	if g.db == nil {
		return nil, false
	}
	var lat, lng sql.NullFloat64
	var prec string
	var at time.Time
	err := g.db.QueryRowContext(ctx, `
		SELECT latitude, longitude, geo_precision, looked_up_at
		  FROM app.geocode_cache WHERE query_key = $1`, key).Scan(&lat, &lng, &prec, &at)
	if err != nil {
		return nil, false
	}
	if prec == "" || !lat.Valid || !lng.Valid {
		return nil, time.Since(at) < negativeTTL
	}
	return &Result{Lat: lat.Float64, Lng: lng.Float64, Precision: prec}, true
}

// store guarda a resposta (inclusive o "não localizado"). Falha do cache não
// é falha da busca: só deixa de poupar a próxima consulta.
func (g *Geocoder) store(ctx context.Context, key string, res *Result) {
	if g.db == nil {
		return
	}
	var lat, lng any
	prec := ""
	if res != nil {
		lat, lng, prec = res.Lat, res.Lng, res.Precision
	}
	_, _ = g.db.ExecContext(ctx, `
		INSERT INTO app.geocode_cache (query_key, latitude, longitude, geo_precision)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (query_key) DO UPDATE
		   SET latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
		       geo_precision = EXCLUDED.geo_precision, looked_up_at = now()`,
		key, lat, lng, prec)
}

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
	// PrecisionCity: nem a rua nem o bairro estão no mapa (o comum na zona
	// rural) — o ponto é a sede do município.
	PrecisionCity = "cidade"
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
	// AltNeighborhood é o bairro como o relatório escreveu, quando
	// Neighborhood veio corrigido pelo catálogo: o catálogo também erra, e
	// o mapa pode ter a outra grafia.
	AltNeighborhood string
	City            string
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
// cidade; sem resultado, o bairro — ponto aproximado; em zona rural, por
// último, a sede do município. Devolve (nil, nil) quando nada foi
// localizado; erro só para falha do serviço.
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

// name entra na chave do cache: muda quando a busca muda, para as respostas
// guardadas pela regra antiga (sobretudo os "não localizado") não valerem.
func (g *nominatim) name() string { return "nominatim/2" }

// lookup desce os degraus do mais preciso ao mais largo, e para no primeiro
// que achar:
//
//  1. a rua (com o número, se houver), como o relatório a escreve;
//  2. a rua com erro de digitação — procura pelas outras palavras do nome e
//     compara o que o mapa devolve pelo som;
//  3. o bairro: a localidade com esse nome;
//  4. um lugar público que leva o nome do bairro (cemitério, escola, posto
//     de saúde) — onde o mapa não tem o bairro, tem a escola dele;
//  5. a rodovia citada no endereço ("CE 065"), dentro do município;
//  6. a sede do município, o último recurso.
func (g *nominatim) lookup(ctx context.Context, q Query) (*Result, error) {
	steps := []func(context.Context, Query) (*Result, error){
		g.street, g.streetMisspelled, g.neighborhood, g.landmark, g.highway, g.citySeat,
	}
	for _, step := range steps {
		res, err := step(ctx, q)
		if res != nil || err != nil {
			return res, err
		}
	}
	return nil, nil
}

// structured monta a busca por campos, restrita à UF.
func (g *nominatim) structured(p url.Values) url.Values {
	if g.state != "" {
		p.Set("state", g.state)
	}
	return p
}

// free monta a busca em texto livre: "parte, cidade, UF".
func (g *nominatim) free(parts ...string) url.Values {
	if g.state != "" {
		parts = append(parts, g.state)
	}
	return url.Values{"q": {strings.Join(parts, ", ")}}
}

// street procura a rua pelo nome escrito, primeiro por campos e depois em
// texto livre (que entende melhor abreviação e complemento). Endereço com
// mais de um trecho ("CE 065, Avenida Tabatinga") ou de esquina ("Av. 12 com
// Rua 47") é tentado trecho a trecho.
func (g *nominatim) street(ctx context.Context, q Query) (*Result, error) {
	for _, name := range streetParts(q.Street) {
		// A rodovia sozinha ("CE 065") acha um trecho qualquer dela no
		// município: fica para depois do bairro (ver highway).
		if len(streetTokens(reHighway.ReplaceAllString(name, ""))) == 0 {
			continue
		}
		full := name
		if q.Number != "" {
			full = q.Number + " " + name
		}
		for _, p := range []url.Values{
			g.structured(url.Values{"street": {full}, "city": {q.City}}),
			g.free(full, q.City),
		} {
			hits, err := g.search(ctx, p, 1)
			if err != nil {
				return nil, err
			}
			if len(hits) == 0 || hits[0].rank < rankStreet || !hits[0].in(q.City) {
				continue
			}
			prec := PrecisionStreet
			if q.Number != "" && hits[0].rank >= rankDoor {
				prec = PrecisionDoor
			}
			return hits[0].result(prec), nil
		}
	}
	return nil, nil
}

// streetMisspelled recupera a rua escrita com erro. O Nominatim só acha a
// palavra exata, então a busca tira uma palavra do nome por vez — uma delas
// é a errada — e compara as ruas que voltam com o nome inteiro, pelo som e
// com tolerância a uma letra ("Rodrigres" → "Rodrigues").
func (g *nominatim) streetMisspelled(ctx context.Context, q Query) (*Result, error) {
	for _, name := range streetParts(q.Street) {
		want := streetTokens(name)
		if len(want) < 2 {
			continue // sem outra palavra para puxar a rua
		}
		for skip := range want {
			rest := append(append([]string{}, want[:skip]...), want[skip+1:]...)
			hits, err := g.search(ctx, g.free(strings.Join(rest, " "), q.City), misspelledLimit)
			if err != nil {
				return nil, err
			}
			for _, h := range hits {
				if h.rank >= rankStreet && h.category == "highway" && h.in(q.City) && sameName(want, streetTokens(h.name)) {
					return h.result(PrecisionStreet), nil
				}
			}
		}
	}
	return nil, nil
}

// misspelledLimit: quantas ruas pedir ao procurar a que foi escrita errada.
const misspelledLimit = 40

func (g *nominatim) neighborhood(ctx context.Context, q Query) (*Result, error) {
	for _, nb := range q.neighborhoods() {
		// featureType=settlement: só localidade (bairro, distrito, povoado),
		// nunca uma rua ou comércio que por acaso tenha o nome do bairro.
		p := g.free(nb, q.City)
		p.Set("featureType", "settlement")
		hits, err := g.search(ctx, p, 1)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 && hits[0].rank > rankCityLimit && hits[0].rank < rankStreet {
			return hits[0].result(PrecisionNeighborhood), nil
		}
	}
	return nil, nil
}

// neighborhoods são as grafias do bairro a tentar, a corrigida primeiro.
func (q Query) neighborhoods() []string {
	var out []string
	for _, nb := range []string{q.Neighborhood, q.AltNeighborhood} {
		if nb != "" && (len(out) == 0 || intel.Normalize(nb) != intel.Normalize(out[0])) {
			out = append(out, nb)
		}
	}
	return out
}

// landmark procura um lugar público com o nome do bairro: "Cemitério da
// Tabatinga" diz onde fica a Tabatinga que o mapa não tem como bairro.
// Comércio e imóvel particular não servem — o nome deles não é o do lugar.
func (g *nominatim) landmark(ctx context.Context, q Query) (*Result, error) {
	for _, nb := range q.neighborhoods() {
		want := placeTokens(nb)
		if len(want) == 0 || isRural(nb) {
			continue
		}
		hits, err := g.search(ctx, g.free(nb, q.City), landmarkLimit)
		if err != nil {
			return nil, err
		}
		var best *hit
		for i := range hits {
			h := &hits[i]
			if h.rank <= rankCityLimit || !h.in(q.City) || !containsName(placeTokens(h.name), want) {
				continue
			}
			if w := landmarkWeight(h); w > 0 && (best == nil || w > landmarkWeight(best)) {
				best = h
			}
		}
		if best != nil {
			return best.result(PrecisionNeighborhood), nil
		}
	}
	return nil, nil
}

const landmarkLimit = 15

// landmarkWeight diz se o lugar serve de referência para o bairro, e o
// quanto: 0 = não serve (particular ou sem relação com o lugar).
func landmarkWeight(h *hit) int {
	switch h.category {
	case "place", "boundary":
		return 4 // a própria localidade, que a busca de bairro não classificou
	case "landuse":
		if h.typ == "cemetery" {
			return 3
		}
	case "amenity":
		switch h.typ {
		case "grave_yard", "school", "kindergarten", "clinic", "hospital", "health_post":
			return 3
		case "college", "university", "police", "fire_station", "townhall", "community_centre",
			"public_building", "courthouse", "post_office", "library", "bus_station", "marketplace",
			"social_facility", "place_of_worship":
			return 2
		}
	case "leisure":
		switch h.typ {
		case "park", "stadium", "pitch", "playground":
			return 1
		}
	case "office":
		if h.typ == "government" {
			return 2
		}
	}
	return 0
}

var reHighway = regexp.MustCompile(`(?i)\b(CE|BR)[\s-]*(\d{3})\b`)

// highway procura a rodovia que o endereço cita. O ponto é o do trecho que o
// mapa devolve dentro do município — a rua está certa, a altura não.
func (g *nominatim) highway(ctx context.Context, q Query) (*Result, error) {
	m := reHighway.FindStringSubmatch(q.Street)
	if m == nil {
		return nil, nil
	}
	hits, err := g.search(ctx, g.free(strings.ToUpper(m[1])+"-"+m[2], q.City), 1)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 && hits[0].rank >= rankStreet && hits[0].category == "highway" && hits[0].in(q.City) {
		return hits[0].result(PrecisionStreet), nil
	}
	return nil, nil
}

// citySeat devolve a sede do município: o que sobra quando nem a rua, nem o
// bairro, nem uma referência dele estão no mapa (o caso comum da zona
// rural).
func (g *nominatim) citySeat(ctx context.Context, q Query) (*Result, error) {
	hits, err := g.search(ctx, g.structured(url.Values{"city": {q.City}}), 1)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 && hits[0].rank <= rankCityLimit {
		return hits[0].result(PrecisionCity), nil
	}
	return nil, nil
}

// isRural reconhece o bairro que o relatório usa para o que fica fora da
// cidade: "ZONA RURAL", "Área Rural", "RURAL".
func isRural(neighborhood string) bool {
	for _, w := range strings.Fields(intel.Normalize(neighborhood)) {
		if w == "rural" {
			return true
		}
	}
	return false
}

// ─── Nomes ───

var reCorner = regexp.MustCompile(`(?i)\s+(com|c/|x|esquina com|esq\.? com)\s+`)

// streetParts separa os trechos de um endereço composto, na ordem em que
// valem a tentativa: o endereço inteiro, depois cada trecho.
func streetParts(street string) []string {
	street = strings.TrimSpace(street)
	if street == "" {
		return nil
	}
	out := []string{street}
	for _, byComma := range strings.Split(street, ",") {
		for _, p := range reCorner.Split(byComma, -1) {
			if p = strings.TrimSpace(p); p != "" && p != street {
				out = append(out, p)
			}
		}
	}
	return out
}

// Palavras que não distinguem uma rua da outra: o tipo da via e os
// conectivos.
var streetStop = map[string]bool{
	"rua": true, "r": true, "avenida": true, "av": true, "travessa": true, "tv": true, "trav": true,
	"estrada": true, "estr": true, "rodovia": true, "rod": true, "alameda": true, "al": true,
	"praca": true, "pc": true, "beco": true, "vila": true, "via": true, "largo": true,
	"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true,
}

// Idem para o nome de um lugar: o que ele é não entra na comparação com o
// nome do bairro.
var placeStop = map[string]bool{
	"de": true, "da": true, "do": true, "das": true, "dos": true, "e": true,
	"bairro": true, "conjunto": true, "conj": true, "loteamento": true, "residencial": true,
	"parque": true, "vila": true, "sitio": true, "distrito": true, "localidade": true,
}

func tokens(s string, stop map[string]bool) []string {
	var out []string
	for _, t := range strings.Fields(intel.Normalize(s)) {
		if !stop[t] {
			out = append(out, t)
		}
	}
	return out
}

func streetTokens(s string) []string { return tokens(s, streetStop) }
func placeTokens(s string) []string  { return tokens(s, placeStop) }

// sameName: os dois nomes têm as mesmas palavras, na ordem, a menos de som e
// de uma letra. O mapa pode trazer o nome mais completo que o relatório
// ("Raimundo Justini de Morais" para "Raimundo Justine"), nunca o contrário.
func sameName(want, have []string) bool {
	if len(want) < 2 || len(have) < len(want) {
		return false
	}
	i := 0
	for _, h := range have {
		if i < len(want) && intel.NearSound(want[i], h) {
			i++
		}
	}
	return i == len(want)
}

// containsName: todas as palavras de want aparecem em have (a menos de som e
// de uma letra), em qualquer posição.
func containsName(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if intel.NearSound(w, h) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

type hit struct {
	lat, lng float64
	rank     int
	name     string // nome do lugar ("Rua Quintino Cunha")
	display  string // endereço completo, com bairro, cidade e UF
	category string // classe no mapa: highway, place, amenity, landuse…
	typ      string
}

func (h *hit) result(precision string) *Result {
	return &Result{Lat: h.lat, Lng: h.lng, Precision: precision}
}

// in confere se o lugar fica na cidade pedida: a busca em texto livre pode
// devolver o homônimo de outro município.
func (h *hit) in(city string) bool {
	c := intel.Normalize(city)
	if c == "" || h.display == "" {
		return true
	}
	for _, part := range strings.Split(h.display, ",") {
		if intel.Normalize(part) == c {
			return true
		}
	}
	return false
}

func (g *nominatim) search(ctx context.Context, p url.Values, limit int) ([]hit, error) {
	p.Set("format", "jsonv2")
	p.Set("limit", strconv.Itoa(limit))
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
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		PlaceRank   int    `json:"place_rank"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Category    string `json:"category"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&items); err != nil {
		return nil, fmt.Errorf("geocodificador: resposta inválida: %w", err)
	}
	out := make([]hit, 0, len(items))
	for _, it := range items {
		lat, err1 := strconv.ParseFloat(it.Lat, 64)
		lng, err2 := strconv.ParseFloat(it.Lon, 64)
		if err1 != nil || err2 != nil {
			return nil, errors.New("geocodificador: coordenada inválida na resposta")
		}
		out = append(out, hit{lat: lat, lng: lng, rank: it.PlaceRank, name: it.Name,
			display: it.DisplayName, category: it.Category, typ: it.Type})
	}
	return out, nil
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

		AltNeighborhood: sp(q.AltNeighborhood),
	}
}

// ─── Cache ───

func (g *Geocoder) cacheKey(q Query) string {
	parts := []string{q.Street, q.Number, q.Neighborhood, q.City, g.state}
	if q.AltNeighborhood != "" {
		parts = append(parts, q.AltNeighborhood)
	}
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

// Resolução do link curto do Google Maps que acompanha o relatório de CVLI.
//
// O relatório traz "LINK NO GOOGLE MAPS: https://maps.app.goo.gl/…" — um
// encurtador, sem coordenada na URL. Para plotar o ponto no mapa do crime é
// preciso seguir o redirect até a URL longa, que carrega o par lat/long.
//
// Isso faz o servidor bater no Google a cada relatório importado. Duas
// consequências assumidas: (a) o IP da agência aparece no log do Google
// associado ao ponto do crime; (b) o cadastro passa a depender de rede
// externa — por isso a falha aqui NUNCA bloqueia a importação, só devolve
// erro para virar aviso na tela. INCIDENT_MAPS_RESOLVER=off desliga.
package incidents

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Erros da resolução — todos tratáveis como aviso, nunca fatais.
var (
	ErrMapsDisabled   = errors.New("resolução de link do Maps desligada")
	ErrMapsHost       = errors.New("link não é do Google Maps")
	ErrMapsUnresolved = errors.New("não foi possível extrair coordenadas do link")
)

// mapsHosts é a allowlist de destinos. Sem ela o endpoint viraria um SSRF:
// o analista cola qualquer URL e o servidor a busca de dentro da rede.
var mapsHosts = map[string]bool{
	"maps.app.goo.gl":    true,
	"goo.gl":             true,
	"g.co":               true,
	"maps.google.com":    true,
	"maps.google.com.br": true,
	"www.google.com":     true,
	"google.com":         true,
	"www.google.com.br":  true,
	"google.com.br":      true,
}

// maxMapsBody limita a leitura do corpo — as coordenadas aparecem no início
// do HTML; ler a página inteira só gastaria memória.
const maxMapsBody = 512 << 10

// MapsResolverEnabled diz se a resolução está ligada (padrão: ligada).
func MapsResolverEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("INCIDENT_MAPS_RESOLVER")), "off")
}

// MapsPoint é o que o link rendeu. Nem todo link do Maps aponta um ponto:
// muitos resolvem para uma BUSCA por endereço (…/maps?q=Rua+X&ftid=…), sem
// coordenada nenhuma. Nesses casos Address carrega o endereço buscado, que
// pelo menos permite ao analista conferir o local — e às vezes revela que o
// link aponta para outro município.
type MapsPoint struct {
	Latitude  float64
	Longitude float64
	HasCoords bool
	Address   string
	FinalURL  string
}

// ResolveMapsLink segue o link curto e devolve o ponto. Procura primeiro na
// URL final (o caminho normal: /@lat,lng ou !3d!4d) e, se não achar, nos
// marcadores do HTML.
func ResolveMapsLink(ctx context.Context, rawURL string) (MapsPoint, error) {
	var out MapsPoint
	if !MapsResolverEnabled() {
		return out, ErrMapsDisabled
	}
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return out, ErrMapsHost
	}
	// Coordenada já na URL colada dispensa qualquer requisição.
	if la, lo, ok := coordsFromText(rawURL); ok {
		return MapsPoint{Latitude: la, Longitude: lo, HasCoords: true, FinalURL: rawURL}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return out, ErrMapsHost
	}
	if req.URL.Scheme != "https" || !mapsHosts[strings.ToLower(req.URL.Hostname())] {
		return out, ErrMapsHost
	}
	// Sem User-Agent o Google devolve uma página degradada, sem as
	// coordenadas no corpo.
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9")

	resp, err := mapsClient.Do(req)
	if err != nil {
		return out, fmt.Errorf("resolver link: %w", err)
	}
	defer resp.Body.Close()

	if resp.Request != nil && resp.Request.URL != nil {
		final := resp.Request.URL
		out.FinalURL = final.String()
		out.Address = queryAddress(final)
		if la, lo, ok := coordsFromText(out.FinalURL); ok {
			out.Latitude, out.Longitude, out.HasCoords = la, lo, true
			return out, nil
		}
	}
	// No HTML só valem os padrões ancorados: a página está cheia de floats
	// que passariam por um par lat/long solto.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxMapsBody))
	if la, lo, ok := coordsFromPage(string(body)); ok {
		out.Latitude, out.Longitude, out.HasCoords = la, lo, true
		return out, nil
	}
	return out, ErrMapsUnresolved
}

// queryAddress devolve o endereço buscado (parâmetro q=/query=) quando o
// link resolveu para uma busca em vez de um ponto.
func queryAddress(u *url.URL) string {
	for _, k := range []string{"q", "query", "daddr"} {
		if v := strings.TrimSpace(u.Query().Get(k)); v != "" {
			// Coordenada em q= já teria sido capturada pelos padrões; aqui só
			// interessa texto.
			if _, _, ok := coordsFromText(v); ok {
				continue
			}
			return v
		}
	}
	return ""
}

// mapsClient revalida o host a cada salto: um redirect é um destino novo, e
// a allowlist não vale nada se só o primeiro for conferido.
var mapsClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return errors.New("redirects em excesso")
		}
		if req.URL.Scheme != "https" || !mapsHosts[strings.ToLower(req.URL.Hostname())] {
			return fmt.Errorf("redirect para host não permitido: %s", req.URL.Host)
		}
		return nil
	},
}

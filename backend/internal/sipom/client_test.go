package sipom

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// Vetores calculados à parte (Python hmac/hashlib), na fórmula do manual.
func TestSign(t *testing.T) {
	body := []byte(`{"origem":{"id":"x"}}`)
	if got, want := Sign("segredo-teste", "1760890000", "POST", PathReceber, body),
		"sha256=e0cdc16b43a9dd064acf07218308d73fcab118d200db1a4afff1292ca0e2b8f5"; got != want {
		t.Fatalf("POST: %s, quer %s", got, want)
	}
	// Sem corpo: sha256("").
	if got, want := Sign("segredo-teste", "1760890000", "GET", PathPing, nil),
		"sha256=2064e90611e33769758e139d510178a883902a738f4c3e87273b5128ef483456"; got != want {
		t.Fatalf("GET: %s, quer %s", got, want)
	}
}

func testClient(srv *httptest.Server, now time.Time) *Client {
	return &Client{BaseURL: srv.URL, Key: "cli_teste", Secret: "segredo-teste", HTTP: srv.Client(),
		Now: func() time.Time { return now }, Retries: 2, Sleep: func(time.Duration) {}}
}

func TestReceberAssinaOCorpoEnviado(t *testing.T) {
	now := time.Unix(1760890000, 0)
	body := []byte(`{"origem":{"id":"x"}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		ts := r.Header.Get("X-SIPOM-Timestamp")
		if r.Header.Get("X-SIPOM-Cliente") != "cli_teste" || ts != strconv.FormatInt(now.Unix(), 10) ||
			r.Header.Get("X-SIPOM-Assinatura") != Sign("segredo-teste", ts, r.Method, r.URL.Path, got) ||
			string(got) != string(body) || r.URL.Path != PathReceber {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"erro":"assinatura","codigo":"assinatura_invalida"}`))
			return
		}
		_, _ = w.Write([]byte(`{"protocolo":"p-1","situacao":"criado","identificador":"2026A1B2C3D4",
			"avisos":[{"campo":"composicao.2.matricula","codigo":"matricula_nao_encontrada"}]}`))
	}))
	defer srv.Close()
	res, err := testClient(srv, now).Receber(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Situacao != SituacaoCriado || res.Identificador != "2026A1B2C3D4" || len(res.Avisos) != 1 {
		t.Fatalf("resultado: %+v", res)
	}
}

func TestReceberRepeteSo429e500(t *testing.T) {
	for _, tc := range []struct {
		status, calls int
	}{{http.StatusTooManyRequests, 3}, {http.StatusInternalServerError, 3}, {http.StatusUnprocessableEntity, 1}, {http.StatusConflict, 1}} {
		calls := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"erro":"x","codigo":"validacao","erros":{"ocorrencia.data_hora":"inválida"}}`))
		}))
		_, err := testClient(srv, time.Now()).Receber(context.Background(), []byte(`{}`))
		srv.Close()
		var ae *APIError
		if !errors.As(err, &ae) || ae.Status != tc.status || calls != tc.calls {
			t.Fatalf("status %d: err=%v calls=%d", tc.status, err, calls)
		}
		if tc.status == http.StatusUnprocessableEntity && ae.Erros["ocorrencia.data_hora"] != "inválida" {
			t.Fatalf("erros por campo: %+v", ae.Erros)
		}
	}
}

func TestReceberRecusaCorpoGrande(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:1"}
	_, err := c.Receber(context.Background(), make([]byte, MaxBodyBytes+1))
	var ae *APIError
	if !errors.As(err, &ae) || ae.Codigo != ErrCorpoGrande {
		t.Fatalf("err=%v", err)
	}
}

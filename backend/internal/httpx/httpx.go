// Package httpx fornece utilitários de JSON e envelope de resposta padrão Belia.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// Envelope é o formato padrão de resposta da API: { success, data, message, errors }.
type Envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

// WriteJSON escreve um envelope JSON com o status indicado.
func WriteJSON(w http.ResponseWriter, status int, v Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// OK responde 200 com data.
func OK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, Envelope{Success: true, Data: data})
}

// Created responde 201 com data.
func Created(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, Envelope{Success: true, Data: data})
}

// NoContent responde 204.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error responde com status e mensagem genérica (não vaza detalhes internos).
func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, Envelope{Success: false, Message: message})
}

// Decode lê e valida um corpo JSON limitado a 1 MiB.
func Decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("corpo vazio")
		}
		return err
	}
	if dec.More() {
		return errors.New("corpo deve conter um único objeto JSON")
	}
	return nil
}

// ClientIP devolve o IP do cliente. Tenta primeiro X-Forwarded-For (primeiro
// IP da lista — o cliente original em cadeia de proxies) e X-Real-IP. Em
// produção, espera-se que o ingress (Caddy/nginx/Cloudflare) seta esses
// headers. Em dev sem reverse proxy frontal eles normalmente não chegam e
// caímos no RemoteAddr (que em Docker é o IP da bridge interna).
//
// Em produção crítica conviria validar que a request veio de subnet confiável
// antes de honrar os headers (proxy chain trust). Fica de lição de casa
// quando expusermos a API publicamente.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return stripPort(r.RemoteAddr)
}

// stripPort remove ":port" do final, tratando IPv6 "[::1]:8080".
func stripPort(addr string) string {
	if addr == "" {
		return ""
	}
	if addr[0] == '[' {
		if end := strings.IndexByte(addr, ']'); end > 0 {
			return addr[1:end]
		}
	}
	if i := strings.LastIndexByte(addr, ':'); i >= 0 && strings.Count(addr, ":") == 1 {
		return addr[:i]
	}
	return addr
}

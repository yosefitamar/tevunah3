// Package httpx fornece utilitários de JSON e envelope de resposta padrão Belia.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

// ClientIP devolve o IP do cliente; em dev isolado é apenas o RemoteAddr.
// Em produção atrás de proxy, deve ser substituído por parsing de X-Forwarded-For/X-Real-IP confiável.
func ClientIP(r *http.Request) string {
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

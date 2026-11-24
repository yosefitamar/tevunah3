package ai

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

var (
	// ErrKeyFormat: não parece uma chave de API Anthropic.
	ErrKeyFormat = errors.New("isto não parece uma chave da API Anthropic (começa com sk-ant-)")
	// ErrKeyAdmin: chave de administração não chama modelos.
	ErrKeyAdmin = errors.New("esta é uma chave de administração (sk-ant-admin…); use uma chave de API comum")
	// ErrKeyRejected: a API recusou a chave (401).
	ErrKeyRejected = errors.New("a Anthropic recusou a chave — confira se ela está ativa e foi copiada inteira")
	// ErrKeyForbidden: a chave existe mas não pode listar modelos (403).
	ErrKeyForbidden = errors.New("a chave não tem permissão para usar os modelos")
	// ErrUpstream: a API não respondeu como esperado (rede, 5xx, limite).
	ErrUpstream = errors.New("não foi possível falar com a API da Anthropic agora — tente de novo")
)

// verifyTimeout limita a verificação: listar modelos responde em ~1s.
const verifyTimeout = 20 * time.Second

// CheckKeyFormat faz a checagem local antes de ir à API.
func CheckKeyFormat(k string) error {
	if strings.HasPrefix(k, "sk-ant-admin") {
		return ErrKeyAdmin
	}
	if !strings.HasPrefix(k, "sk-ant-") || len(k) < 30 || len(k) > 400 || strings.ContainsAny(k, " \t\r\n") {
		return ErrKeyFormat
	}
	return nil
}

// VerifyKey confirma a chave na API (GET /v1/models, que não gera custo) e
// devolve os ids dos modelos ativos que ela enxerga, em ordem alfabética.
func VerifyKey(ctx context.Context, apiKey string) ([]string, error) {
	if err := CheckKeyFormat(apiKey); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(1),
	)
	var ids []string
	pager := client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{Limit: anthropic.Int(1000)})
	for pager.Next() {
		m := pager.Current()
		if string(m.Lifecycle) == "retired" {
			continue
		}
		ids = append(ids, m.ID)
	}
	if err := pager.Err(); err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			switch apierr.StatusCode {
			case 401:
				return nil, ErrKeyRejected
			case 403:
				return nil, ErrKeyForbidden
			}
		}
		return nil, ErrUpstream
	}
	sort.Strings(ids)
	return ids, nil
}

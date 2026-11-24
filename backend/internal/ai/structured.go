package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

var (
	// ErrRefused: o modelo recusou responder (stop_reason refusal).
	ErrRefused = errors.New("a IA recusou responder a esta solicitação")
	// ErrTruncated: a resposta passou do limite de tokens e veio cortada.
	ErrTruncated = errors.New("a resposta da IA veio incompleta — tente de novo")
	// ErrBadOutput: a resposta não é o JSON pedido.
	ErrBadOutput = errors.New("a IA respondeu fora do formato esperado — tente de novo")
	// ErrRateLimited: a conta Anthropic do usuário atingiu o limite.
	ErrRateLimited = errors.New("limite de uso da sua conta Anthropic atingido — aguarde um pouco e tente de novo")
)

const structuredTimeout = 120 * time.Second

// StructuredRequest é uma chamada que devolve JSON num schema fixo
// (structured outputs: output_config.format). System fica em cache: é a
// parte estável (instruções); Prompt é o caso.
type StructuredRequest struct {
	// Feature identifica a funcionalidade no registro de uso (ai_usage).
	Feature string
	System  string
	Prompt  string
	// Schema é o JSON Schema da resposta (objetos com additionalProperties
	// false, sem minimum/maxLength — limites do structured outputs).
	Schema    map[string]any
	MaxTokens int
	// Effort: low | medium | high (vazio = o padrão do modelo).
	Effort string
}

// StructuredResult é a resposta: o JSON e o que a chamada consumiu.
type StructuredResult struct {
	JSON  json.RawMessage
	Model string
	Usage Usage
}

// Structured chama o modelo do usuário com a chave dele e grava o uso (e o
// custo) em ai_usage — inclusive quando a resposta não serve, porque foi
// cobrada.
func (r *Repo) Structured(ctx context.Context, userID string, req StructuredRequest) (*StructuredResult, error) {
	apiKey, model, err := r.Credentials(ctx, userID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, structuredTimeout)
	defer cancel()

	client := anthropic.NewClient(option.WithAPIKey(apiKey), option.WithMaxRetries(2))
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8000
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		System: []anthropic.TextBlockParam{{
			Text:         req.System,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: req.Schema},
		},
	}
	if req.Effort != "" {
		params.OutputConfig.Effort = anthropic.OutputConfigEffort(req.Effort)
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		var apierr *anthropic.Error
		if errors.As(err, &apierr) {
			switch apierr.StatusCode {
			case 401:
				return nil, ErrKeyRejected
			case 403:
				return nil, ErrKeyForbidden
			case 429:
				return nil, ErrRateLimited
			}
			log.Printf("ai structured %s: %d %s", req.Feature, apierr.StatusCode, apierr.Error())
		} else {
			log.Printf("ai structured %s: %v", req.Feature, err)
		}
		return nil, ErrUpstream
	}

	out := &StructuredResult{Model: model, Usage: Usage{
		UserID:           userID,
		Model:            model,
		Feature:          req.Feature,
		InputTokens:      int(resp.Usage.InputTokens),
		OutputTokens:     int(resp.Usage.OutputTokens),
		CacheWriteTokens: int(resp.Usage.CacheCreationInputTokens),
		CacheReadTokens:  int(resp.Usage.CacheReadInputTokens),
	}}
	// O uso é gravado num contexto próprio: a chamada já foi cobrada, mesmo
	// que o pedido do navegador tenha sido cancelado no meio.
	recCtx, recCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer recCancel()
	if err := r.RecordUsage(recCtx, out.Usage); err != nil {
		log.Printf("ai usage %s: %v", req.Feature, err)
	}

	switch resp.StopReason {
	case anthropic.StopReasonRefusal:
		return out, ErrRefused
	case anthropic.StopReasonMaxTokens:
		return out, ErrTruncated
	}
	var text strings.Builder
	for _, block := range resp.Content {
		if b, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(b.Text)
		}
	}
	raw := strings.TrimSpace(text.String())
	if raw == "" || !json.Valid([]byte(raw)) {
		return out, fmt.Errorf("%w (%d bytes)", ErrBadOutput, len(raw))
	}
	out.JSON = json.RawMessage(raw)
	return out, nil
}

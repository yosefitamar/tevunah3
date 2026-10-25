package sipom

// Cliente da API de integração do SIPOM ("API de integração SIPOM —
// Referência", 2026-10-19): só escrita, base /api/integracoes, cada
// requisição assinada com HMAC-SHA256. Não há sessão, cookie nem CSRF.
//
//	texto      = timestamp + "\n" + MÉTODO + "\n" + caminho + "\n" + sha256_hex(corpo)
//	assinatura = "sha256=" + hmac_sha256_hex(segredo, texto)
//
// Cada assinatura vale uma vez (anti-replay de 10 min): toda tentativa sai
// com timestamp novo. O segredo nunca trafega.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase        = "/api/integracoes"
	PathPing       = apiBase + "/ping"
	PathReceber    = apiBase + "/ocorrencias/receber"
	MaxBodyBytes   = 20 << 20 // acima disto o SIPOM responde 413
	maxRespBytes   = 1 << 20
	defaultTimeout = 60 * time.Second
)

// Situações devolvidas pelo receber.
const (
	SituacaoCriado       = "criado"
	SituacaoAtualizado   = "atualizado"
	SituacaoSemAlteracao = "sem_alteracao"
)

// Client fala com o SIPOM. Zero valor não serve: use NewClientFromEnv.
type Client struct {
	BaseURL string // https://sipom.pm.ce.gov.br
	Key     string // chave pública (cli_...)
	Secret  string
	HTTP    *http.Client
	// Now permite fixar o relógio nos testes.
	Now func() time.Time
	// Retries: tentativas extras em 429, 500 e falha de rede.
	Retries int
	// Sleep permite pular a espera nos testes.
	Sleep func(time.Duration)
}

// NewClientFromEnv lê SIPOM_BASE_URL, SIPOM_CLIENT_KEY e SIPOM_SECRET. Sem
// chave ou segredo devolve nil: a integração fica desligada e o envio volta
// a ser marcado à mão.
func NewClientFromEnv() *Client {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("SIPOM_BASE_URL")), "/")
	key, secret := strings.TrimSpace(os.Getenv("SIPOM_CLIENT_KEY")), os.Getenv("SIPOM_SECRET")
	if base == "" || key == "" || secret == "" {
		return nil
	}
	return &Client{
		BaseURL: base, Key: key, Secret: secret,
		HTTP: &http.Client{Timeout: defaultTimeout},
		Now:  time.Now, Retries: 2, Sleep: time.Sleep,
	}
}

// Sign devolve a assinatura do manual para a requisição.
func Sign(secret, ts, method, path string, body []byte) string {
	sum := sha256.Sum256(body)
	text := ts + "\n" + strings.ToUpper(method) + "\n" + path + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(text))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// Aviso é um item de "avisos" da resposta: o envio foi aceito, mas algo
// ficou de fora (matrícula não encontrada, numeral não numérico).
type Aviso struct {
	Campo  string `json:"campo"`
	Codigo string `json:"codigo"`
}

// Resultado é a resposta 200 do receber.
type Resultado struct {
	Protocolo     string  `json:"protocolo"`
	Situacao      string  `json:"situacao"`
	Identificador string  `json:"identificador"` // número da ocorrência no SIPOM
	Avisos        []Aviso `json:"avisos"`
}

// APIError é a resposta de erro do SIPOM (ou a falha de rede, Status 0).
type APIError struct {
	Status   int               `json:"-"`
	Mensagem string            `json:"erro"`
	Codigo   string            `json:"codigo"`
	Erros    map[string]string `json:"erros,omitempty"`
	// RetryAfter: espera pedida no 429.
	RetryAfter time.Duration `json:"-"`
}

func (e *APIError) Error() string {
	if e.Status == 0 {
		return "SIPOM inacessível: " + e.Mensagem
	}
	msg := fmt.Sprintf("SIPOM %d %s: %s", e.Status, e.Codigo, e.Mensagem)
	for campo, m := range e.Erros {
		msg += fmt.Sprintf("; %s: %s", campo, m)
	}
	return msg
}

// Retryable diz se vale reenviar sem mudar nada (429, 500, rede).
func (e *APIError) Retryable() bool {
	return e.Status == 0 || e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// Códigos de erro do manual.
const (
	ErrRequisicaoInvalida = "requisicao_invalida" // 400
	ErrAssinaturaInvalida = "assinatura_invalida" // 401
	ErrSemPermissao       = "sem_permissao"       // 403
	ErrConflito           = "conflito"            // 409: ocorrência excluída no SIPOM
	ErrCorpoGrande        = "corpo_grande"        // 413
	ErrValidacao          = "validacao"           // 422
	ErrLimiteExcedido     = "limite_excedido"     // 429
	ErrInterno            = "erro_interno"        // 500
)

// Ping testa a assinatura; não grava nada no SIPOM. Devolve o nome do
// cliente como o SIPOM o conhece.
func (c *Client) Ping(ctx context.Context) (string, error) {
	var out struct {
		OK      bool   `json:"ok"`
		Cliente string `json:"cliente"`
	}
	if err := c.do(ctx, http.MethodGet, PathPing, nil, &out); err != nil {
		return "", err
	}
	if !out.OK {
		return "", &APIError{Status: http.StatusOK, Mensagem: "ping sem ok"}
	}
	return out.Cliente, nil
}

// Receber cria ou atualiza a ocorrência. body é o JSON já serializado: é
// ele, byte a byte, que vai assinado.
func (c *Client) Receber(ctx context.Context, body []byte) (*Resultado, error) {
	if len(body) > MaxBodyBytes {
		return nil, &APIError{Status: http.StatusRequestEntityTooLarge, Codigo: ErrCorpoGrande,
			Mensagem: fmt.Sprintf("corpo de %d bytes acima do limite de 20 MB", len(body))}
	}
	var out Resultado
	if err := c.do(ctx, http.MethodPost, PathReceber, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// do envia com as tentativas extras. Reenviar é seguro: o SIPOM identifica
// a ocorrência por origem.id.
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	var err error
	for attempt := 0; ; attempt++ {
		err = c.once(ctx, method, path, body, out)
		var ae *APIError
		if err == nil || !errors.As(err, &ae) || !ae.Retryable() || attempt >= c.Retries {
			return err
		}
		wait := ae.RetryAfter
		if wait <= 0 {
			wait = time.Duration(attempt+1) * 2 * time.Second
		}
		if wait > 30*time.Second {
			return err // espera longa demais para quem está na tela
		}
		if ctx.Err() != nil {
			return err
		}
		c.Sleep(wait)
	}
}

func (c *Client) once(ctx context.Context, method, path string, body []byte, out any) error {
	if body == nil {
		body = []byte{}
	}
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	ts := strconv.FormatInt(c.Now().Unix(), 10)
	req.Header.Set("X-SIPOM-Cliente", c.Key)
	req.Header.Set("X-SIPOM-Timestamp", ts)
	req.Header.Set("X-SIPOM-Assinatura", Sign(c.Secret, ts, method, u.EscapedPath(), body))
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &APIError{Mensagem: err.Error()}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return &APIError{Mensagem: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		ae := &APIError{Status: resp.StatusCode}
		if json.Unmarshal(raw, ae) != nil || ae.Mensagem == "" {
			ae.Mensagem = strings.TrimSpace(string(raw))
			if len(ae.Mensagem) > 300 {
				ae.Mensagem = ae.Mensagem[:300]
			}
		}
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			ae.RetryAfter = time.Duration(s) * time.Second
		}
		return ae
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("resposta do SIPOM ilegível: %w", err)
	}
	return nil
}

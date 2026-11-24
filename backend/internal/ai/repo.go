package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/secretbox"
)

// ErrNoKey: o usuário não cadastrou chave.
var ErrNoKey = errors.New("nenhuma chave da API Anthropic cadastrada")

// ErrInvalidModel: modelo fora do catálogo.
var ErrInvalidModel = errors.New("modelo não oferecido")

// Settings é a configuração de um usuário. A chave em si nunca sai daqui.
type Settings struct {
	HasKey          bool
	KeyHint         string
	KeyVerifiedAt   *time.Time
	AvailableModels []string
	Model           string
}

// Repo persiste em app.user_ai_settings (chave cifrada com box) e
// app.ai_usage (gasto). tz é o fuso da agência: "mês" e "ano" do gasto são
// os do calendário de lá.
type Repo struct {
	db  *sql.DB
	box *secretbox.Box
	tz  string
}

func New(db *sql.DB, box *secretbox.Box, tz string) *Repo {
	if tz == "" {
		tz = "America/Fortaleza"
	}
	return &Repo{db: db, box: box, tz: tz}
}

// SecretsReady: sem cofre configurado não se guarda chave.
func (r *Repo) SecretsReady() bool { return r.box.Ready() }

// Get devolve a configuração (padrões quando o usuário nunca salvou).
func (r *Repo) Get(ctx context.Context, userID string) (*Settings, error) {
	s := &Settings{Model: DefaultModel, AvailableModels: []string{}}
	var hint, model sql.NullString
	var verified sql.NullTime
	var models []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT api_key_hint, key_verified_at, to_jsonb(available_models), model
		  FROM app.user_ai_settings WHERE user_id = $1`, userID,
	).Scan(&hint, &verified, &models, &model)
	if errors.Is(err, sql.ErrNoRows) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	s.HasKey = hint.Valid
	s.KeyHint = hint.String
	if verified.Valid {
		t := verified.Time
		s.KeyVerifiedAt = &t
	}
	if len(models) > 0 {
		_ = json.Unmarshal(models, &s.AvailableModels)
	}
	if model.Valid && FindModel(model.String) != nil {
		s.Model = model.String
	}
	return s, nil
}

// SaveKey cifra e grava a chave já verificada, com os modelos que ela vê.
func (r *Repo) SaveKey(ctx context.Context, userID, apiKey string, available []string) error {
	sealed, err := r.box.Seal([]byte(apiKey), []byte(userID))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO app.user_ai_settings
		  (user_id, api_key_enc, api_key_hint, key_verified_at, available_models)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (user_id) DO UPDATE SET
		  api_key_enc = EXCLUDED.api_key_enc,
		  api_key_hint = EXCLUDED.api_key_hint,
		  key_verified_at = EXCLUDED.key_verified_at,
		  available_models = EXCLUDED.available_models,
		  updated_at = now()`,
		userID, sealed, KeyHint(apiKey), nonNil(available))
	return err
}

// MarkVerified registra uma nova verificação da chave já gravada.
func (r *Repo) MarkVerified(ctx context.Context, userID string, available []string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.user_ai_settings
		   SET key_verified_at = now(), available_models = $2, updated_at = now()
		 WHERE user_id = $1 AND api_key_enc IS NOT NULL`, userID, nonNil(available))
	return err
}

// ClearKey remove a chave (anula as colunas; o modelo fica).
func (r *Repo) ClearKey(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.user_ai_settings
		   SET api_key_enc = NULL, api_key_hint = NULL, key_verified_at = NULL,
		       available_models = '{}', updated_at = now()
		 WHERE user_id = $1`, userID)
	return err
}

// SaveModel grava o modelo escolhido.
func (r *Repo) SaveModel(ctx context.Context, userID, model string) error {
	if FindModel(model) == nil {
		return ErrInvalidModel
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app.user_ai_settings (user_id, model) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET model = EXCLUDED.model, updated_at = now()`,
		userID, model)
	return err
}

// Credentials devolve a chave decifrada e o modelo do usuário — o ponto de
// entrada das features de IA. ErrNoKey se não houver chave.
func (r *Repo) Credentials(ctx context.Context, userID string) (apiKey, model string, err error) {
	var sealed []byte
	var m sql.NullString
	err = r.db.QueryRowContext(ctx, `
		SELECT api_key_enc, model FROM app.user_ai_settings WHERE user_id = $1`, userID,
	).Scan(&sealed, &m)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && len(sealed) == 0) {
		return "", "", ErrNoKey
	}
	if err != nil {
		return "", "", err
	}
	plain, err := r.box.Open(sealed, []byte(userID))
	if err != nil {
		return "", "", fmt.Errorf("chave da API: %w", err)
	}
	model = DefaultModel
	if m.Valid && FindModel(m.String) != nil {
		model = m.String
	}
	return string(plain), model, nil
}

// ─── Gasto ───────────────────────────────────────────────────────────

// Usage é o que uma feature de IA registra depois de cada chamada à API:
// os campos de usage da resposta (input_tokens, output_tokens,
// cache_creation_input_tokens, cache_read_input_tokens).
type Usage struct {
	UserID           string
	Model            string
	Feature          string
	InputTokens      int
	OutputTokens     int
	CacheWriteTokens int
	CacheReadTokens  int
}

// RecordUsage grava a chamada com o custo calculado pelo preço de tabela do
// modelo neste momento. Modelo fora do catálogo é recusado: sem preço não há
// como dizer quanto custou.
func (r *Repo) RecordUsage(ctx context.Context, u Usage) error {
	m := FindModel(u.Model)
	if m == nil {
		return ErrInvalidModel
	}
	if u.Feature == "" {
		return errors.New("ai.RecordUsage: feature vazia")
	}
	cost := m.Cost(u.InputTokens, u.OutputTokens, u.CacheWriteTokens, u.CacheReadTokens)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app.ai_usage
		  (user_id, model, feature, input_tokens, output_tokens,
		   cache_write_tokens, cache_read_tokens, cost_usd)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.UserID, u.Model, u.Feature, u.InputTokens, u.OutputTokens,
		u.CacheWriteTokens, u.CacheReadTokens, cost)
	return err
}

// Period soma o uso de um período.
type Period struct {
	CostUSD      float64 `json:"cost_usd"`
	Requests     int     `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
}

// Spending é o gasto do usuário no mês e no ano correntes (calendário da
// agência).
type Spending struct {
	Month Period `json:"month"`
	Year  Period `json:"year"`
}

// Spending soma o gasto do mês e do ano correntes.
func (r *Repo) Spending(ctx context.Context, userID string) (*Spending, error) {
	return r.spendingQ(ctx, r.db, userID)
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (r *Repo) spendingQ(ctx context.Context, q queryRower, userID string) (*Spending, error) {
	var s Spending
	err := q.QueryRowContext(ctx, `
		WITH b AS (
		  SELECT date_trunc('month', now() AT TIME ZONE $2) AT TIME ZONE $2 AS m0,
		         date_trunc('year',  now() AT TIME ZONE $2) AT TIME ZONE $2 AS y0
		)
		SELECT
		  COALESCE(SUM(u.cost_usd) FILTER (WHERE u.created_at >= b.m0), 0)::float8,
		  COUNT(u.id) FILTER (WHERE u.created_at >= b.m0),
		  COALESCE(SUM(u.input_tokens + u.cache_write_tokens + u.cache_read_tokens) FILTER (WHERE u.created_at >= b.m0), 0),
		  COALESCE(SUM(u.output_tokens) FILTER (WHERE u.created_at >= b.m0), 0),
		  COALESCE(SUM(u.cost_usd), 0)::float8,
		  COUNT(u.id),
		  COALESCE(SUM(u.input_tokens + u.cache_write_tokens + u.cache_read_tokens), 0),
		  COALESCE(SUM(u.output_tokens), 0)
		  FROM b
		  LEFT JOIN app.ai_usage u ON u.user_id = $1 AND u.created_at >= b.y0`,
		userID, r.tz,
	).Scan(
		&s.Month.CostUSD, &s.Month.Requests, &s.Month.InputTokens, &s.Month.OutputTokens,
		&s.Year.CostUSD, &s.Year.Requests, &s.Year.InputTokens, &s.Year.OutputTokens,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// KeyHint mostra só o prefixo e os 4 últimos caracteres: "sk-ant-…a1B2".
func KeyHint(k string) string {
	k = strings.TrimSpace(k)
	if len(k) <= 12 {
		return "…"
	}
	prefix := "sk-ant-"
	if !strings.HasPrefix(k, prefix) {
		prefix = k[:4]
	}
	return prefix + "…" + k[len(k)-4:]
}

func nonNil(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

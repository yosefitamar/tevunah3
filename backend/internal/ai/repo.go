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

// ErrInvalidUsage: perfil de uso fora dos limites.
var ErrInvalidUsage = errors.New("perfil de uso inválido")

// UsageProfile alimenta a estimativa de custo da tela.
type UsageProfile struct {
	RequestsPerDay int     `json:"requests_per_day"`
	WorkDays       int     `json:"work_days_per_month"`
	InputTokens    int     `json:"input_tokens_per_request"`
	OutputTokens   int     `json:"output_tokens_per_request"`
	USDBRL         float64 `json:"usd_brl"`
}

// DefaultUsage é o perfil inicial: um analista com uso moderado.
var DefaultUsage = UsageProfile{
	RequestsPerDay: 40, WorkDays: 22, InputTokens: 4000, OutputTokens: 1200, USDBRL: 5.50,
}

func (u UsageProfile) valid() bool {
	return u.RequestsPerDay >= 0 && u.RequestsPerDay <= 100000 &&
		u.WorkDays >= 1 && u.WorkDays <= 31 &&
		u.InputTokens >= 0 && u.InputTokens <= 1_000_000 &&
		u.OutputTokens >= 0 && u.OutputTokens <= 128_000 &&
		u.USDBRL > 0 && u.USDBRL <= 100
}

// Settings é a configuração de um usuário. A chave em si nunca sai daqui.
type Settings struct {
	HasKey          bool
	KeyHint         string
	KeyVerifiedAt   *time.Time
	AvailableModels []string
	Model           string
	Usage           UsageProfile
	UpdatedAt       *time.Time
}

// Repo persiste em app.user_ai_settings, cifrando a chave com box.
type Repo struct {
	db  *sql.DB
	box *secretbox.Box
}

func New(db *sql.DB, box *secretbox.Box) *Repo { return &Repo{db: db, box: box} }

// SecretsReady: sem cofre configurado não se guarda chave.
func (r *Repo) SecretsReady() bool { return r.box.Ready() }

// Get devolve a configuração (padrões quando o usuário nunca salvou).
func (r *Repo) Get(ctx context.Context, userID string) (*Settings, error) {
	s := &Settings{Model: DefaultModel, Usage: DefaultUsage, AvailableModels: []string{}}
	var hint, model sql.NullString
	var verified, updated sql.NullTime
	var usage []byte
	var models []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT api_key_hint, key_verified_at, to_jsonb(available_models), model, usage_profile, updated_at
		  FROM app.user_ai_settings WHERE user_id = $1`, userID,
	).Scan(&hint, &verified, &models, &model, &usage, &updated)
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
	if updated.Valid {
		t := updated.Time
		s.UpdatedAt = &t
	}
	if len(models) > 0 {
		_ = json.Unmarshal(models, &s.AvailableModels)
	}
	if model.Valid && FindModel(model.String) != nil {
		s.Model = model.String
	}
	if len(usage) > 2 { // "{}" = nunca salvo
		var u UsageProfile
		if err := json.Unmarshal(usage, &u); err == nil && u.valid() {
			s.Usage = u
		}
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

// ClearKey remove a chave (anula as colunas; modelo e perfil ficam).
func (r *Repo) ClearKey(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.user_ai_settings
		   SET api_key_enc = NULL, api_key_hint = NULL, key_verified_at = NULL,
		       available_models = '{}', updated_at = now()
		 WHERE user_id = $1`, userID)
	return err
}

// SaveSettings grava o modelo escolhido e o perfil de uso.
func (r *Repo) SaveSettings(ctx context.Context, userID, model string, usage UsageProfile) error {
	if FindModel(model) == nil {
		return ErrInvalidModel
	}
	if !usage.valid() {
		return ErrInvalidUsage
	}
	raw, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO app.user_ai_settings (user_id, model, usage_profile)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
		  model = EXCLUDED.model, usage_profile = EXCLUDED.usage_profile, updated_at = now()`,
		userID, model, string(raw))
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

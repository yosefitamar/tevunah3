package ai

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Orientações dos analistas para a IA (app.ai_guidance). Pendente vale só
// para o autor; aprovada (pelo administrador) vale para todos.

const (
	GuidancePending  = "pendente"
	GuidanceApproved = "aprovada"
	GuidanceRejected = "recusada"
	GuidanceDisabled = "desativada"
)

// GuidanceTopics são os temas aceitos.
var GuidanceTopics = map[string]string{
	"natureza":   "Natureza",
	"bairro":     "Bairro",
	"sexo":       "Sexo dos envolvidos",
	"apreensoes": "Apreensões",
	"geral":      "Geral",
}

var (
	ErrGuidanceNotFound = errors.New("orientação não encontrada")
	ErrGuidanceText     = errors.New("a orientação deve ter de 5 a 1000 caracteres")
	ErrGuidanceTopic    = errors.New("tema inválido")
	ErrGuidanceState    = errors.New("a orientação não está mais pendente")
)

// Guidance é uma orientação.
type Guidance struct {
	ID           string     `json:"id"`
	Feature      string     `json:"feature"`
	Topic        string     `json:"topic"`
	CidadeID     *int       `json:"cidade_id"`
	Body         string     `json:"body"`
	Status       string     `json:"status"`
	SourceOccID  *string    `json:"source_occurrence_id"`
	CreatedBy    string     `json:"created_by"`
	AuthorName   string     `json:"author_name"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DecidedBy    *string    `json:"decided_by"`
	DeciderName  string     `json:"decider_name"`
	DecidedAt    *time.Time `json:"decided_at"`
	DecisionNote string     `json:"decision_note"`
	Uses         int        `json:"uses"`
	LastUsedAt   *time.Time `json:"last_used_at"`
}

const guidanceCols = `
	g.id, g.feature, g.topic, g.cidade_id, g.body, g.status, g.source_occurrence_id,
	g.created_by, COALESCE(NULLIF(a.display_name, ''), a.email), g.created_at, g.updated_at,
	g.decided_by, COALESCE(NULLIF(d.display_name, ''), d.email, ''), g.decided_at, g.decision_note,
	g.uses, g.last_used_at`

const guidanceFrom = `
	FROM app.ai_guidance g
	JOIN app.users a ON a.id = g.created_by
	LEFT JOIN app.users d ON d.id = g.decided_by`

func scanGuidance(sc interface{ Scan(...any) error }) (*Guidance, error) {
	var g Guidance
	err := sc.Scan(&g.ID, &g.Feature, &g.Topic, &g.CidadeID, &g.Body, &g.Status, &g.SourceOccID,
		&g.CreatedBy, &g.AuthorName, &g.CreatedAt, &g.UpdatedAt,
		&g.DecidedBy, &g.DeciderName, &g.DecidedAt, &g.DecisionNote, &g.Uses, &g.LastUsedAt)
	return &g, err
}

func (r *Repo) queryGuidance(ctx context.Context, where string, args ...any) ([]Guidance, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+guidanceCols+guidanceFrom+` WHERE `+where+
		` ORDER BY (g.status = 'pendente') DESC, g.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Guidance{}
	for rows.Next() {
		g, err := scanGuidance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

// ActiveGuidance são as orientações que valem para o usuário na feature: as
// aprovadas e as pendentes dele.
func (r *Repo) ActiveGuidance(ctx context.Context, feature, userID string) ([]Guidance, error) {
	return r.queryGuidance(ctx,
		`g.feature = $1 AND (g.status = 'aprovada' OR (g.status = 'pendente' AND g.created_by = $2))`,
		feature, userID)
}

// ListGuidance lista para a tela: o administrador vê todas; os demais, as
// aprovadas e as próprias (em qualquer situação).
func (r *Repo) ListGuidance(ctx context.Context, feature, userID string, all bool) ([]Guidance, error) {
	if all {
		return r.queryGuidance(ctx, `g.feature = $1`, feature)
	}
	return r.queryGuidance(ctx, `g.feature = $1 AND (g.status = 'aprovada' OR g.created_by = $2)`, feature, userID)
}

// GetGuidance lê uma orientação.
func (r *Repo) GetGuidance(ctx context.Context, id string) (*Guidance, error) {
	g, err := scanGuidance(r.db.QueryRowContext(ctx, `SELECT `+guidanceCols+guidanceFrom+` WHERE g.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGuidanceNotFound
	}
	return g, err
}

// GuidanceInput é o que o analista escreve.
type GuidanceInput struct {
	Topic       string
	CidadeID    *int
	Body        string
	SourceOccID *string
}

func (in *GuidanceInput) normalize() error {
	in.Body = strings.Join(strings.Fields(in.Body), " ")
	if n := len([]rune(in.Body)); n < 5 || n > 1000 {
		return ErrGuidanceText
	}
	if _, ok := GuidanceTopics[in.Topic]; !ok {
		return ErrGuidanceTopic
	}
	return nil
}

// CreateGuidance grava a orientação como pendente.
func (r *Repo) CreateGuidance(ctx context.Context, feature, userID string, in GuidanceInput) (*Guidance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO app.ai_guidance (feature, topic, cidade_id, body, source_occurrence_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		feature, in.Topic, in.CidadeID, in.Body, in.SourceOccID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetGuidance(ctx, id)
}

// UpdateGuidance troca o texto, o tema e a cidade — só enquanto pendente.
func (r *Repo) UpdateGuidance(ctx context.Context, id string, in GuidanceInput) (*Guidance, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.ai_guidance SET topic = $2, cidade_id = $3, body = $4, updated_at = now()
		 WHERE id = $1 AND status = 'pendente'`, id, in.Topic, in.CidadeID, in.Body)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrGuidanceState
	}
	return r.GetGuidance(ctx, id)
}

// DecideGuidance muda a situação. Aprovar e recusar só de pendente;
// desativar, de pendente ou aprovada.
func (r *Repo) DecideGuidance(ctx context.Context, id, status, userID, note string) (*Guidance, error) {
	from := []string{GuidancePending}
	if status == GuidanceDisabled {
		from = append(from, GuidanceApproved)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.ai_guidance
		   SET status = $2, decided_by = $3, decided_at = now(), decision_note = $4, updated_at = now()
		 WHERE id = $1 AND status = ANY($5)`,
		id, status, userID, strings.TrimSpace(note), from)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrGuidanceState
	}
	return r.GetGuidance(ctx, id)
}

// MarkGuidanceUsed conta o uso das orientações citadas numa sugestão.
func (r *Repo) MarkGuidanceUsed(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.ai_guidance SET uses = uses + 1, last_used_at = now() WHERE id = ANY($1)`, ids)
	return err
}

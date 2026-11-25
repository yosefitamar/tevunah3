package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"
)

// Análises guardadas (app.ai_analyses): a última de cada ocorrência aparece
// sem nova consulta; a seguinte recebe a anterior com o resultado da revisão.

var ErrAnalysisNotFound = errors.New("análise não encontrada")

// Analysis é uma análise guardada.
type Analysis struct {
	ID           string
	Feature      string
	OccurrenceID string
	PreviousID   *string
	CreatedBy    string
	AuthorName   string
	CreatedAt    time.Time
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Sections     []string
	Suggestions  json.RawMessage
	Fingerprint  string
	Applied      []int
	Rejected     []int
	Failed       map[string]string // posição → erro
	ResultAt     *time.Time
}

// NewAnalysis é o que se grava ao fim de uma consulta.
type NewAnalysis struct {
	Feature, OccurrenceID, UserID string
	PreviousID                    *string
	Model                         string
	InputTokens, OutputTokens     int
	CostUSD                       float64
	Sections                      []string
	Suggestions                   any
	Fingerprint                   string
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// SaveAnalysis grava a análise e devolve o id e a hora.
func (r *Repo) SaveAnalysis(ctx context.Context, n NewAnalysis) (string, time.Time, error) {
	if n.Sections == nil {
		n.Sections = []string{}
	}
	body, err := json.Marshal(n.Suggestions)
	if err != nil {
		return "", time.Time{}, err
	}
	var id string
	var at time.Time
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO app.ai_analyses
		  (feature, occurrence_id, previous_id, created_by, model, input_tokens, output_tokens,
		   cost_usd, sections, suggestions, fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)
		RETURNING id, created_at`,
		n.Feature, n.OccurrenceID, n.PreviousID, n.UserID, n.Model, n.InputTokens, n.OutputTokens,
		n.CostUSD, string(mustJSON(n.Sections)), string(body), n.Fingerprint).Scan(&id, &at)
	return id, at, err
}

// LatestAnalysis devolve a última análise da ocorrência (nil se não há).
func (r *Repo) LatestAnalysis(ctx context.Context, feature, occurrenceID string) (*Analysis, error) {
	var a Analysis
	var sections, sugg, applied, rejected, failed string
	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.feature, a.occurrence_id, a.previous_id, a.created_by,
		       COALESCE(NULLIF(u.display_name, ''), u.email, ''), a.created_at, a.model,
		       a.input_tokens, a.output_tokens, a.cost_usd::float8, a.sections::text, a.suggestions::text,
		       a.fingerprint, a.applied::text, a.rejected::text, a.failed::text, a.result_at
		  FROM app.ai_analyses a
		  LEFT JOIN app.users u ON u.id = a.created_by
		 WHERE a.feature = $1 AND a.occurrence_id = $2
		 ORDER BY a.created_at DESC
		 LIMIT 1`, feature, occurrenceID).Scan(
		&a.ID, &a.Feature, &a.OccurrenceID, &a.PreviousID, &a.CreatedBy, &a.AuthorName, &a.CreatedAt, &a.Model,
		&a.InputTokens, &a.OutputTokens, &a.CostUSD, &sections, &sugg,
		&a.Fingerprint, &applied, &rejected, &failed, &a.ResultAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.Suggestions = json.RawMessage(sugg)
	_ = json.Unmarshal([]byte(sections), &a.Sections)
	_ = json.Unmarshal([]byte(applied), &a.Applied)
	_ = json.Unmarshal([]byte(rejected), &a.Rejected)
	a.Failed = map[string]string{}
	_ = json.Unmarshal([]byte(failed), &a.Failed)
	return &a, nil
}

// MergeResult junta à análise o resultado de uma revisão (o analista pode
// aplicar em mais de uma vez): aplicadas somam; recusada que depois foi
// aplicada deixa de ser recusada; a falha de um item é a mais recente e some
// quando ele é aplicado.
func (a *Analysis) MergeResult(applied, rejected []int, failed map[string]string) {
	set := func(xs ...[]int) map[int]bool {
		m := map[int]bool{}
		for _, x := range xs {
			for _, v := range x {
				m[v] = true
			}
		}
		return m
	}
	list := func(m map[int]bool) []int {
		out := make([]int, 0, len(m))
		for v := range m {
			out = append(out, v)
		}
		sort.Ints(out)
		return out
	}
	ap := set(a.Applied, applied)
	rj := set(a.Rejected, rejected)
	for v := range ap {
		delete(rj, v)
	}
	if a.Failed == nil {
		a.Failed = map[string]string{}
	}
	for k, v := range failed {
		a.Failed[k] = v
	}
	for k := range a.Failed {
		if n, err := strconv.Atoi(k); err == nil && ap[n] {
			delete(a.Failed, k)
		}
	}
	a.Applied, a.Rejected = list(ap), list(rj)
}

// SaveAnalysisResult grava o resultado já juntado (MergeResult). A impressão
// digital passa a ser a da ficha depois das correções — a mudança veio da
// própria análise.
func (r *Repo) SaveAnalysisResult(ctx context.Context, a *Analysis, fingerprint string) error {
	if fingerprint == "" {
		fingerprint = a.Fingerprint
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.ai_analyses
		   SET applied = $3::jsonb, rejected = $4::jsonb, failed = $5::jsonb, fingerprint = $6, result_at = now()
		 WHERE id = $1 AND occurrence_id = $2`,
		a.ID, a.OccurrenceID, string(mustJSON(a.Applied)), string(mustJSON(a.Rejected)), string(mustJSON(a.Failed)), fingerprint)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAnalysisNotFound
	}
	a.Fingerprint = fingerprint
	return nil
}

// AnalysisByID lê uma análise da ocorrência.
func (r *Repo) AnalysisByID(ctx context.Context, feature, occurrenceID, id string) (*Analysis, error) {
	a, err := r.LatestAnalysis(ctx, feature, occurrenceID)
	if err != nil {
		return nil, err
	}
	if a == nil || a.ID != id {
		return nil, ErrAnalysisNotFound // só a última recebe resultado
	}
	return a, nil
}

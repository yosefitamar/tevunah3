// Package approvals encapsula o ciclo de vida de pending_approvals (4-eyes).
//
// Fluxo:
//
//  1. Solicitante chama um endpoint sensível. Servidor detecta dual_approval
//     pela matriz e cria Approval com status="pending".
//  2. Aprovador (papel exigido por required_approver_role e != solicitante)
//     decide via Approve/Reject. Approve dispara o executor da ação no chamador.
//  3. Solicitante pode Cancel enquanto status="pending".
//  4. Aprovações com expires_at no passado são marcadas como "expired"
//     preguiçosamente em consultas (TODO: job de varredura para produção).
package approvals

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound       = errors.New("aprovação não encontrada")
	ErrInvalidStatus  = errors.New("status inválido para a operação")
	ErrSelfApproval   = errors.New("aprovador não pode ser o solicitante")
	ErrNotApprover    = errors.New("usuário não tem o papel exigido para aprovar")
	ErrExpired        = errors.New("aprovação expirada")
	ErrNotRequester   = errors.New("apenas o solicitante pode cancelar")
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusExpired   Status = "expired"
	StatusCancelled Status = "cancelled"
)

// Approval é o registro completo de pending_approvals.
type Approval struct {
	ID                   string
	Action               string
	ResourceType         *string
	ResourceID           *string
	Payload              json.RawMessage
	RequestedBy          string
	RequestedAt          time.Time
	RequiredApproverRole string
	Status               Status
	DecidedBy            *string
	DecidedAt            *time.Time
	DecisionReason       *string
	ExpiresAt            time.Time
}

// IsTerminal indica que a aprovação não está mais pendente.
func (a *Approval) IsTerminal() bool { return a.Status != StatusPending }

// Expired indica se já passou da expiração (independente do campo status).
func (a *Approval) Expired() bool { return a.ExpiresAt.Before(time.Now()) }

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// CreateInput descreve a solicitação de aprovação.
type CreateInput struct {
	Action               string
	RequestedBy          string
	RequiredApproverRole string
	ResourceType         *string
	ResourceID           *string
	Payload              json.RawMessage
	ExpiresIn            time.Duration // default 48h se zero
}

func (r *Repo) Create(ctx context.Context, in CreateInput) (*Approval, error) {
	expires := in.ExpiresIn
	if expires <= 0 {
		expires = 48 * time.Hour
	}
	if len(in.Payload) == 0 {
		in.Payload = json.RawMessage("{}")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO app.pending_approvals
		  (action, resource_type, resource_id, payload, requested_by,
		   required_approver_role, expires_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, now() + $7::interval)
		RETURNING id, action, resource_type, resource_id, payload, requested_by,
		          requested_at, required_approver_role, status, decided_by,
		          decided_at, decision_reason, expires_at`,
		in.Action,
		nilStr(in.ResourceType),
		nilStr(in.ResourceID),
		string(in.Payload),
		in.RequestedBy,
		in.RequiredApproverRole,
		fmt.Sprintf("%d seconds", int(expires.Seconds())),
	)
	return scanRow(row)
}

func (r *Repo) Get(ctx context.Context, id string) (*Approval, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, action, resource_type, resource_id, payload, requested_by,
		       requested_at, required_approver_role, status, decided_by,
		       decided_at, decision_reason, expires_at
		  FROM app.pending_approvals
		 WHERE id = $1`, id)
	return scanRow(row)
}

// Decide muda status para approved|rejected de modo atômico. Retorna ErrInvalidStatus
// se o registro não está mais pendente, ErrSelfApproval se decidedBy == requestedBy.
func (r *Repo) Decide(ctx context.Context, id, decidedBy string, decision Status, reason string) (*Approval, error) {
	if decision != StatusApproved && decision != StatusRejected {
		return nil, fmt.Errorf("decisão inválida: %s", decision)
	}
	// Carrega para validar requester e expirado
	cur, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusPending {
		return nil, ErrInvalidStatus
	}
	if cur.Expired() {
		// marca expirada e devolve erro
		_, _ = r.db.ExecContext(ctx,
			`UPDATE app.pending_approvals SET status='expired'
			   WHERE id=$1 AND status='pending'`, id)
		return nil, ErrExpired
	}
	if cur.RequestedBy == decidedBy {
		return nil, ErrSelfApproval
	}

	var reasonArg any = nil
	if r := strings.TrimSpace(reason); r != "" {
		reasonArg = r
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE app.pending_approvals
		   SET status = $2, decided_by = $3, decided_at = now(), decision_reason = $4
		 WHERE id = $1 AND status = 'pending'
		 RETURNING id, action, resource_type, resource_id, payload, requested_by,
		           requested_at, required_approver_role, status, decided_by,
		           decided_at, decision_reason, expires_at`,
		id, string(decision), decidedBy, reasonArg)
	return scanRow(row)
}

// Cancel só permite o próprio solicitante cancelar enquanto pending.
func (r *Repo) Cancel(ctx context.Context, id, by string, reason string) (*Approval, error) {
	cur, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.Status != StatusPending {
		return nil, ErrInvalidStatus
	}
	if cur.RequestedBy != by {
		return nil, ErrNotRequester
	}
	var reasonArg any = nil
	if r := strings.TrimSpace(reason); r != "" {
		reasonArg = r
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE app.pending_approvals
		   SET status='cancelled', decided_by=$2, decided_at=now(), decision_reason=$3
		 WHERE id=$1 AND status='pending'
		 RETURNING id, action, resource_type, resource_id, payload, requested_by,
		           requested_at, required_approver_role, status, decided_by,
		           decided_at, decision_reason, expires_at`,
		id, by, reasonArg)
	return scanRow(row)
}

// ListOpts controla a listagem.
type ListOpts struct {
	Status        Status // vazio = todos
	RequestedBy   string // filtra solicitante
	ApproverRoles []string // se preenchido, lista apenas onde required_approver_role ∈ esses papéis
	ExcludeUserID string // se preenchido, exclui aprovações desse user_id (para não mostrar próprias)
	Limit         int
	Offset        int
}

type ListResult struct {
	Items []Approval
	Total int
}

func (r *Repo) List(ctx context.Context, opts ListOpts) (*ListResult, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 50
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	rolesArr := "{" + strings.Join(opts.ApproverRoles, ",") + "}"

	args := []any{
		string(opts.Status),
		opts.RequestedBy,
		rolesArr,
		opts.ExcludeUserID,
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM app.pending_approvals
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR requested_by = $2::uuid)
		   AND ($3 = '{}' OR required_approver_role = ANY($3::text[]))
		   AND ($4 = '' OR requested_by <> $4::uuid)`,
		args...,
	).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, action, resource_type, resource_id, payload, requested_by,
		       requested_at, required_approver_role, status, decided_by,
		       decided_at, decision_reason, expires_at
		  FROM app.pending_approvals
		 WHERE ($1 = '' OR status = $1)
		   AND ($2 = '' OR requested_by = $2::uuid)
		   AND ($3 = '{}' OR required_approver_role = ANY($3::text[]))
		   AND ($4 = '' OR requested_by <> $4::uuid)
		 ORDER BY requested_at DESC
		 LIMIT $5 OFFSET $6`,
		append(args, opts.Limit, opts.Offset)...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Approval, 0)
	for rows.Next() {
		a, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ListResult{Items: items, Total: total}, nil
}

// ─────────── helpers ───────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(s *sql.Row) (*Approval, error) {
	return scanGeneric(s)
}
func scanRows(s *sql.Rows) (*Approval, error) {
	return scanGeneric(s)
}

func scanGeneric(s rowScanner) (*Approval, error) {
	var a Approval
	var resType, resID, decBy, decReason sql.NullString
	var decAt sql.NullTime
	var payload []byte
	if err := s.Scan(&a.ID, &a.Action, &resType, &resID, &payload, &a.RequestedBy,
		&a.RequestedAt, &a.RequiredApproverRole, &a.Status, &decBy, &decAt, &decReason,
		&a.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if resType.Valid {
		v := resType.String
		a.ResourceType = &v
	}
	if resID.Valid {
		v := resID.String
		a.ResourceID = &v
	}
	if decBy.Valid {
		v := decBy.String
		a.DecidedBy = &v
	}
	if decAt.Valid {
		t := decAt.Time
		a.DecidedAt = &t
	}
	if decReason.Valid {
		v := decReason.String
		a.DecisionReason = &v
	}
	a.Payload = json.RawMessage(payload)
	return &a, nil
}

func nilStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

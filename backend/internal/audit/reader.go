// Reader: leituras da audit_log para os módulos de Auditoria (gestor/admin).
//
// Conecta como tevunah_app (que tem SELECT em audit.audit_log e em app.users
// para o LEFT JOIN do ator). Reads nunca geram entrada no audit_log para
// evitar amplificação infinita.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound é devolvido por FindByID quando o id não existe.
var ErrNotFound = errors.New("audit entry not found")

// LogEntry é o registro retornado por List/FindByID, enriquecido com
// code/display_name do ator quando disponível.
type LogEntry struct {
	ID                     int64           `json:"id"`
	TS                     time.Time       `json:"ts"`
	ActorUserID            *string         `json:"actor_user_id,omitempty"`
	ActorUserCode          *string         `json:"actor_user_code,omitempty"`
	ActorDisplayName       *string         `json:"actor_display_name,omitempty"`
	ActorSessionID         *string         `json:"actor_session_id,omitempty"`
	ActorIP                *string         `json:"actor_ip,omitempty"`
	ActorTerminal          *string         `json:"actor_terminal,omitempty"`
	ActorUserAgent         *string         `json:"actor_user_agent,omitempty"`
	Action                 string          `json:"action"`
	ResourceType           *string         `json:"resource_type,omitempty"`
	ResourceID             *string         `json:"resource_id,omitempty"`
	ResourceClassification *int            `json:"resource_classification,omitempty"`
	Before                 json.RawMessage `json:"before,omitempty"`
	After                  json.RawMessage `json:"after,omitempty"`
	Reason                 *string         `json:"reason,omitempty"`
	PrevHash               string          `json:"prev_hash"` // hex
	Hash                   string          `json:"hash"`      // hex
}

// ListOpts controla filtros e paginação. Limit é clampado em [1, 200] com
// default 25.
type ListOpts struct {
	Limit        int
	Offset       int
	Action       string // exato; com sufixo '*' vira prefixo (LIKE)
	ActorID      string // uuid
	ResourceType string
	ResourceID   string
	From, To     *time.Time
	Search       string // lower-match em action, code/email do ator, resource_id
	SortBy       string // "id"|"ts"|"action"|"actor"|"resource"; default "id"
	SortDir      string // "asc"|"desc"; default "desc"
}

// auditSortable mapeia campos públicos para SQL. Whitelist obrigatória.
var auditSortable = map[string]string{
	"id":       "l.id",
	"ts":       "l.ts",
	"action":   "l.action",
	"actor":    "coalesce(u.code, '~')", // ordena por code do ator; '~' joga NULLs pro fim
	"resource": "(coalesce(l.resource_type,'') || ':' || coalesce(l.resource_id,''))",
}

type ListResult struct {
	Items []LogEntry
	Total int64
}

// Reader consulta audit.audit_log. Usa o pool do tevunah_app.
type Reader struct {
	db *sql.DB
}

func NewReader(db *sql.DB) *Reader { return &Reader{db: db} }

// List devolve a página solicitada (mais recentes primeiro) e o total absoluto
// do filtro.
func (r *Reader) List(ctx context.Context, opts ListOpts) (ListResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	var (
		where []string
		args  []any
	)
	ph := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if a := strings.TrimSpace(opts.Action); a != "" {
		if strings.HasSuffix(a, "*") {
			where = append(where, "l.action LIKE "+ph(strings.TrimSuffix(a, "*")+"%"))
		} else {
			where = append(where, "l.action = "+ph(a))
		}
	}
	if id := strings.TrimSpace(opts.ActorID); id != "" {
		where = append(where, "l.actor_user_id = "+ph(id)+"::uuid")
	}
	if rt := strings.TrimSpace(opts.ResourceType); rt != "" {
		where = append(where, "l.resource_type = "+ph(rt))
	}
	if rid := strings.TrimSpace(opts.ResourceID); rid != "" {
		where = append(where, "l.resource_id = "+ph(rid))
	}
	if opts.From != nil {
		where = append(where, "l.ts >= "+ph(*opts.From))
	}
	if opts.To != nil {
		where = append(where, "l.ts < "+ph(*opts.To))
	}
	if s := strings.TrimSpace(opts.Search); s != "" {
		needle := "%" + strings.ToLower(s) + "%"
		p := ph(needle)
		where = append(where,
			"(lower(l.action) LIKE "+p+
				" OR lower(coalesce(u.code,'')) LIKE "+p+
				" OR lower(coalesce(u.email,'')) LIKE "+p+
				" OR lower(coalesce(l.resource_id,'')) LIKE "+p+")")
	}

	whereSQL := "1=1"
	if len(where) > 0 {
		whereSQL = strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT count(*)
		  FROM audit.audit_log l
		  LEFT JOIN app.users u ON u.id = l.actor_user_id
		 WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	col, ok := auditSortable[opts.SortBy]
	if !ok {
		col = "l.id"
	}
	dir := "DESC"
	if strings.ToLower(opts.SortDir) == "asc" {
		dir = "ASC"
	}

	limitPH := ph(limit)
	offsetPH := ph(offset)

	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.ts,
		       l.actor_user_id::text, u.code, u.display_name,
		       l.actor_session_id, host(l.actor_ip), l.actor_terminal, l.actor_user_agent,
		       l.action, l.resource_type, l.resource_id, l.resource_classification,
		       l.before, l.after, l.reason,
		       encode(l.prev_hash, 'hex'), encode(l.hash, 'hex')
		  FROM audit.audit_log l
		  LEFT JOIN app.users u ON u.id = l.actor_user_id
		 WHERE `+whereSQL+`
		 ORDER BY `+col+` `+dir+`, l.id DESC
		 LIMIT `+limitPH+` OFFSET `+offsetPH, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]LogEntry, 0, limit)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	return ListResult{Items: items, Total: total}, nil
}

// FindByID devolve a entrada completa pelo id.
func (r *Reader) FindByID(ctx context.Context, id int64) (*LogEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT l.id, l.ts,
		       l.actor_user_id::text, u.code, u.display_name,
		       l.actor_session_id, host(l.actor_ip), l.actor_terminal, l.actor_user_agent,
		       l.action, l.resource_type, l.resource_id, l.resource_classification,
		       l.before, l.after, l.reason,
		       encode(l.prev_hash, 'hex'), encode(l.hash, 'hex')
		  FROM audit.audit_log l
		  LEFT JOIN app.users u ON u.id = l.actor_user_id
		 WHERE l.id = $1`, id)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// scanRow generaliza Row e Rows (ambos têm Scan com a mesma assinatura).
type scanRow interface {
	Scan(dest ...any) error
}

func scanEntry(s scanRow) (LogEntry, error) {
	var (
		e                        LogEntry
		beforeBytes, afterBytes  []byte
	)
	if err := s.Scan(
		&e.ID, &e.TS,
		&e.ActorUserID, &e.ActorUserCode, &e.ActorDisplayName,
		&e.ActorSessionID, &e.ActorIP, &e.ActorTerminal, &e.ActorUserAgent,
		&e.Action, &e.ResourceType, &e.ResourceID, &e.ResourceClassification,
		&beforeBytes, &afterBytes, &e.Reason,
		&e.PrevHash, &e.Hash,
	); err != nil {
		return LogEntry{}, err
	}
	if len(beforeBytes) > 0 {
		e.Before = json.RawMessage(beforeBytes)
	}
	if len(afterBytes) > 0 {
		e.After = json.RawMessage(afterBytes)
	}
	return e, nil
}

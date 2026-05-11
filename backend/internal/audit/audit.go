// Package audit insere entradas no trilho append-only audit.audit_log.
//
// Esta camada conecta como tevunah_audit_writer (role que só tem INSERT na tabela).
// O trigger BEFORE INSERT calcula id, prev_hash e hash; UPDATE/DELETE/TRUNCATE são
// bloqueados pelos triggers da migration 00003.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
)

// Entry é o conjunto de campos exposto à camada de negócio.
// Campos NULL devem ser passados como nil em strings opcionais via *string.
type Entry struct {
	ActorUserID            *string // UUID do ator; nil em LOGIN_DENIED com email inválido
	ActorSessionID         *string
	ActorIP                *string // formato inet aceito por Postgres (ex.: "192.168.1.1")
	ActorTerminal          *string
	ActorUserAgent         *string // header User-Agent da requisição HTTP
	Action                 string  // ex.: "user.create", "auth.login", "auth.login_denied"
	ResourceType           *string // ex.: "user", "operation"
	ResourceID             *string
	ResourceClassification *int
	Before                 any
	After                  any
	Reason                 *string
}

// Logger insere no audit_log. Mantém uma conexão Postgres dedicada (writer pool).
type Logger struct {
	db *sql.DB
}

func New(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log persiste a entrada. before/after são serializados como JSONB (NULL se nil).
func (l *Logger) Log(ctx context.Context, e Entry) error {
	beforeJSON, err := toJSON(e.Before)
	if err != nil {
		return err
	}
	afterJSON, err := toJSON(e.After)
	if err != nil {
		return err
	}
	_, err = l.db.ExecContext(ctx, `
		INSERT INTO audit.audit_log
		  (actor_user_id, actor_session_id, actor_ip, actor_terminal, actor_user_agent,
		   action, resource_type, resource_id, resource_classification,
		   before, after, reason)
		VALUES ($1, $2, $3::inet, $4, $5, $6, $7, $8, $9, $10::jsonb, $11::jsonb, $12)`,
		nilUUID(e.ActorUserID), nilStr(e.ActorSessionID), nilStr(e.ActorIP),
		nilStr(e.ActorTerminal), nilStr(e.ActorUserAgent),
		e.Action, nilStr(e.ResourceType), nilStr(e.ResourceID), nilInt(e.ResourceClassification),
		beforeJSON, afterJSON, nilStr(e.Reason),
	)
	return err
}

func toJSON(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func nilStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
func nilInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// nilUUID trata strings vazias como NULL para colunas uuid.
func nilUUID(p *string) any {
	if p == nil || *p == "" {
		return nil
	}
	return *p
}

// Ptr é açúcar para criar ponteiros literais.
func Ptr[T any](v T) *T { return &v }

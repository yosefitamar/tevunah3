// Package settings expõe a tabela singleton app.system_settings.
//
// Modelagem: uma única linha (key = 'singleton'). Get sempre devolve essa
// linha; Update faz UPDATE (não cria). O seed da migration garante
// existência inicial.
package settings

import (
	"context"
	"database/sql"
	"fmt"
)

type Settings struct {
	AgencyName     string
	DocumentTitle  string
	BrasaoPath     string // "" quando nunca houve upload
}

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// Get devolve as configurações atuais. Como há sempre uma linha (seed),
// qualquer erro além de sql.ErrNoRows é inesperado.
func (r *Repo) Get(ctx context.Context) (*Settings, error) {
	var s Settings
	var brasao sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT agency_name, document_title, brasao_path
		  FROM app.system_settings
		 WHERE key = 'singleton'`,
	).Scan(&s.AgencyName, &s.DocumentTitle, &brasao)
	if err != nil {
		return nil, fmt.Errorf("settings get: %w", err)
	}
	if brasao.Valid {
		s.BrasaoPath = brasao.String
	}
	return &s, nil
}

// UpdateText atualiza agency_name e document_title. Não toca em brasao_path
// (esse é atualizado separadamente via SetBrasaoPath após o upload).
func (r *Repo) UpdateText(ctx context.Context, agency, title, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.system_settings
		   SET agency_name    = $1,
		       document_title = $2,
		       updated_at     = now(),
		       updated_by     = $3
		 WHERE key = 'singleton'`,
		agency, title, actor)
	return err
}

// SetBrasaoPath grava o filename do brasão recém-uploadado. Caller é
// responsável por já ter persistido o arquivo em PHOTO_DIR.
func (r *Repo) SetBrasaoPath(ctx context.Context, filename, actor string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.system_settings
		   SET brasao_path = $1,
		       updated_at  = now(),
		       updated_by  = $2
		 WHERE key = 'singleton'`,
		filename, actor)
	return err
}

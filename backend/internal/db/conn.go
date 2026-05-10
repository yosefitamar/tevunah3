// Package db provê helpers de conexão Postgres para os comandos do projeto.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open abre uma conexão SQL ao Postgres com pool padrão e ping de verificação.
func Open(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DSN vazio")
	}
	d, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	d.SetMaxOpenConns(10)
	d.SetMaxIdleConns(2)
	d.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return d, nil
}

// Env devolve o valor de uma env var ou default.
func Env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

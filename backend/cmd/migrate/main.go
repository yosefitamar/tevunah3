// migrate aplica/reverte migrations no Postgres usando goose com SQL embarcado.
//
// Uso:
//
//	migrate up          # aplica todas as migrations pendentes
//	migrate down        # reverte a última migration
//	migrate status      # lista migrations e status
//	migrate version     # versão atual
//	migrate redo        # down + up da última
//	migrate reset       # reverte tudo (USE COM CUIDADO)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	embedDB "github.com/belia/tevunah/backend/db"
	"github.com/belia/tevunah/backend/internal/db"
	"github.com/pressly/goose/v3"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "uso: migrate <up|down|status|version|redo|reset|create NAME>")
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	cmd := args[0]

	dsn := db.Env("MIGRATIONS_DATABASE_URL", db.Env("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("MIGRATIONS_DATABASE_URL (ou DATABASE_URL) não definido")
	}

	conn, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("conexão: %v", err)
	}
	defer conn.Close()

	goose.SetBaseFS(embedDB.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("dialect: %v", err)
	}

	dir := "migrations"
	switch cmd {
	case "up":
		err = goose.Up(conn, dir)
	case "down":
		err = goose.Down(conn, dir)
	case "status":
		err = goose.Status(conn, dir)
	case "version":
		err = goose.Version(conn, dir)
	case "redo":
		err = goose.Redo(conn, dir)
	case "reset":
		err = goose.Reset(conn, dir)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatalf("%s: %v", cmd, err)
	}
}

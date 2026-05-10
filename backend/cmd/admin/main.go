// admin é a CLI para gestão de administradores fora do app (bootstrap).
//
// Subcomandos:
//
//	admin create     — cria um administrador interativamente (prompts no terminal)
//	admin seed-dev   — em APP_ENV=development, cria o admin de desenvolvimento
//	                  (idempotente: não faz nada se já existir admin ativo)
//
// O comando conecta como o role tevunah_app (APP_DATABASE_URL).
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/belia/tevunah/backend/internal/crypt"
	"github.com/belia/tevunah/backend/internal/db"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "create":
		runCreate(os.Args[2:])
	case "seed-dev":
		runSeedDev(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "uso: admin <create|seed-dev>")
}

func openDB() *sql.DB {
	dsn := db.Env("APP_DATABASE_URL", db.Env("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("APP_DATABASE_URL ou DATABASE_URL não definido")
	}
	conn, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	return conn
}

// generateTOTPSecret devolve 20 bytes aleatórios em base32 (formato RFC 4648
// sem padding), usados como secret de TOTP.
func generateTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// nextAdminCode devolve o próximo código sequencial no formato ADM-NNNN.
func nextAdminCode(ctx context.Context, conn *sql.DB) (string, error) {
	var n int
	err := conn.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(NULLIF(regexp_replace(code, '^ADM-', ''), '')::int), 0)
		  FROM app.users
		 WHERE code LIKE 'ADM-%'`).Scan(&n)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return fmt.Sprintf("ADM-%04d", n+1), nil
}

type newAdminInput struct {
	Email       string
	DisplayName string
	Code        string
	Password    string
	Clearance   int
	Reason      string
}

// createAdmin insere o usuário, vincula ao role administrador e registra audit.
// Não recebe TOTP em texto: gera o secret e devolve para exibir uma única vez.
func createAdmin(ctx context.Context, conn *sql.DB, in newAdminInput) (userID, totpSecret string, err error) {
	if in.Email == "" || in.DisplayName == "" || in.Password == "" {
		return "", "", errors.New("email, nome e senha são obrigatórios")
	}
	if in.Clearance < 1 || in.Clearance > 5 {
		in.Clearance = 5
	}
	if in.Code == "" {
		c, err := nextAdminCode(ctx, conn)
		if err != nil {
			return "", "", fmt.Errorf("código: %w", err)
		}
		in.Code = c
	}

	hash, err := crypt.Hash(in.Password)
	if err != nil {
		return "", "", fmt.Errorf("hash: %w", err)
	}
	secret, err := generateTOTPSecret()
	if err != nil {
		return "", "", fmt.Errorf("totp: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO app.users
		  (code, email, display_name, password_hash, totp_secret, clearance_level, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
		RETURNING id`,
		in.Code, strings.ToLower(in.Email), in.DisplayName, hash, secret, in.Clearance,
	).Scan(&userID)
	if err != nil {
		return "", "", fmt.Errorf("insert user: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO app.user_roles (user_id, role_code) VALUES ($1, 'administrador')`,
		userID,
	); err != nil {
		return "", "", fmt.Errorf("vincular role: %w", err)
	}

	after := map[string]any{
		"code": in.Code, "email": in.Email, "display_name": in.DisplayName,
		"clearance_level": in.Clearance, "roles": []string{"administrador"},
	}
	afterJSON, _ := json.Marshal(after)

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit.audit_log (action, resource_type, resource_id, after, reason)
		VALUES ($1, 'user', $2, $3::jsonb, $4)`,
		"user.create", userID, string(afterJSON), in.Reason,
	); err != nil {
		return "", "", fmt.Errorf("audit: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return userID, secret, nil
}

func prompt(label string, hidden bool) (string, error) {
	fmt.Fprint(os.Stderr, label)
	if hidden {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func runCreate(args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	email := fs.String("email", "", "e-mail do administrador")
	name := fs.String("name", "", "nome de exibição")
	code := fs.String("code", "", "código (opcional; ex.: ADM-0001)")
	clearance := fs.Int("clearance", 5, "nível de clearance (1..5)")
	_ = fs.Parse(args)

	if *email == "" {
		v, err := prompt("E-mail: ", false)
		if err != nil {
			log.Fatal(err)
		}
		*email = v
	}
	if *name == "" {
		v, err := prompt("Nome de exibição: ", false)
		if err != nil {
			log.Fatal(err)
		}
		*name = v
	}
	pass, err := prompt("Senha: ", true)
	if err != nil {
		log.Fatal(err)
	}
	pass2, err := prompt("Confirme a senha: ", true)
	if err != nil {
		log.Fatal(err)
	}
	if pass != pass2 {
		log.Fatal("as senhas não conferem")
	}
	if len(pass) < 12 {
		log.Fatal("senha deve ter ao menos 12 caracteres")
	}

	conn := openDB()
	defer conn.Close()

	ctx := context.Background()
	id, secret, err := createAdmin(ctx, conn, newAdminInput{
		Email: *email, DisplayName: *name, Code: *code,
		Password: pass, Clearance: *clearance, Reason: "cli admin create",
	})
	if err != nil {
		log.Fatalf("falhou: %v", err)
	}

	fmt.Printf(`
✓ Administrador criado.

  id           : %s
  email        : %s
  clearance    : CL-%02d
  TOTP secret  : %s

Configure o secret acima em um app TOTP (Aegis, 1Password, Authy)
ANTES do primeiro login. Ele NÃO será exibido novamente.
`, id, *email, *clearance, secret)
}

func runSeedDev(_ []string) {
	env := db.Env("APP_ENV", "")
	if env != "development" {
		log.Printf("seed-dev: APP_ENV=%q != development — nada a fazer", env)
		return
	}

	conn := openDB()
	defer conn.Close()
	ctx := context.Background()

	var n int
	err := conn.QueryRowContext(ctx, `
		SELECT count(*)
		  FROM app.user_roles ur
		  JOIN app.users u ON u.id = ur.user_id
		 WHERE ur.role_code = 'administrador'
		   AND u.status = 'active'
		   AND u.deleted_at IS NULL`).Scan(&n)
	if err != nil {
		log.Fatalf("contar admins: %v", err)
	}
	if n > 0 {
		log.Printf("seed-dev: já existe(m) %d admin(s) ativo(s) — nada a fazer", n)
		return
	}

	email := db.Env("DEV_ADMIN_EMAIL", "admin@tevunah.local")
	name := db.Env("DEV_ADMIN_NAME", "Administrador Dev")
	pass := db.Env("DEV_ADMIN_PASSWORD", "tevunah-dev-12345")
	code := db.Env("DEV_ADMIN_CODE", "ADM-0001")

	id, secret, err := createAdmin(ctx, conn, newAdminInput{
		Email: email, DisplayName: name, Code: code,
		Password: pass, Clearance: 5, Reason: "seed-dev",
	})
	if err != nil {
		log.Fatalf("seed-dev: %v", err)
	}

	fmt.Printf(`
✓ Admin de desenvolvimento criado.

  id           : %s
  email        : %s
  senha        : %s
  TOTP secret  : %s

Use em DEV apenas. Em produção, rode: ./tevunah admin:create
`, id, email, pass, secret)
}

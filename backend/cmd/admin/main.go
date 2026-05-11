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
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/crypt"
	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/users"
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
	dsn := idb.Env("APP_DATABASE_URL", idb.Env("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("APP_DATABASE_URL ou DATABASE_URL não definido")
	}
	conn, err := idb.Open(dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	return conn
}

// createAdminUser hasheia a senha, gera TOTP, insere usuário+papel e registra audit.
// Devolve o usuário criado e o TOTP secret (mostrado uma única vez).
func createAdminUser(ctx context.Context, repo *users.Repo, auditLog *audit.Logger,
	email, name, password, reason string,
) (*users.User, string, error) {
	code, err := repo.GenerateCode(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("código: %w", err)
	}
	hash, err := crypt.Hash(password)
	if err != nil {
		return nil, "", fmt.Errorf("hash: %w", err)
	}
	secret, err := crypt.NewTOTPSecret()
	if err != nil {
		return nil, "", fmt.Errorf("totp: %w", err)
	}

	u, err := repo.Create(ctx, users.NewUser{
		Code: code, Email: email, DisplayName: name,
		PasswordHash: hash, TOTPSecret: secret,
		ClearanceLevel: 5,
		Roles:          []string{"administrador"},
	})
	if err != nil {
		return nil, "", err
	}

	if err := auditLog.Log(ctx, audit.Entry{
		ActorUserID:  nil, // bootstrap: sem ator humano
		Action:       "user.create",
		ResourceType: audit.Ptr("user"),
		ResourceID:   audit.Ptr(u.ID),
		After: map[string]any{
			"code": u.Code, "email": u.Email, "display_name": u.DisplayName,
			"clearance_level": u.ClearanceLevel, "roles": u.Roles,
		},
		Reason: audit.Ptr(reason),
	}); err != nil {
		log.Printf("aviso: falha ao gravar audit do bootstrap: %v", err)
	}
	return u, secret, nil
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
	repo := users.New(conn)
	auditLog := audit.New(conn)

	u, secret, err := createAdminUser(context.Background(), repo, auditLog,
		*email, *name, pass, "cli admin create")
	if err != nil {
		if errors.Is(err, users.ErrDuplicate) {
			log.Fatal("e-mail ou código já cadastrado")
		}
		log.Fatalf("falhou: %v", err)
	}

	fmt.Printf(`
✓ Administrador criado.

  id           : %s
  code         : %s
  email        : %s
  clearance    : CL-%02d
  TOTP secret  : %s

Configure o secret acima em um app TOTP (Aegis, 1Password, Authy)
ANTES do primeiro login. Ele NÃO será exibido novamente.
`, u.ID, u.Code, u.Email, u.ClearanceLevel, secret)
}

func runSeedDev(_ []string) {
	env := idb.Env("APP_ENV", "")
	if env != "development" {
		log.Printf("seed-dev: APP_ENV=%q != development — nada a fazer", env)
		return
	}

	conn := openDB()
	defer conn.Close()
	ctx := context.Background()

	var n int
	if err := conn.QueryRowContext(ctx, `
		SELECT count(*)
		  FROM app.user_roles ur
		  JOIN app.users u ON u.id = ur.user_id
		 WHERE ur.role_code = 'administrador'
		   AND u.status = 'active'
		   AND u.deleted_at IS NULL`).Scan(&n); err != nil {
		log.Fatalf("contar admins: %v", err)
	}
	if n > 0 {
		log.Printf("seed-dev: já existe(m) %d admin(s) ativo(s) — nada a fazer", n)
		return
	}

	email := idb.Env("DEV_ADMIN_EMAIL", "admin@tevunah.local")
	name := idb.Env("DEV_ADMIN_NAME", "Administrador Dev")
	pass := idb.Env("DEV_ADMIN_PASSWORD", "tevunah-dev-12345")

	repo := users.New(conn)
	auditLog := audit.New(conn)

	u, secret, err := createAdminUser(ctx, repo, auditLog,
		email, name, pass, "seed-dev")
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
`, u.ID, email, pass, secret)
}

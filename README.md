# Belia Tevunah

Sistema de Inteligência — monorepo com backend em Go e frontend em Next.js 15.

```
tevunah2/
├── backend/    Go API + migrations (Postgres) + cmd/admin
└── frontend/   Next.js 15 + TypeScript + Tailwind v4
```

## Stack

- **Backend:** Go 1.23, stdlib `net/http`, [goose](https://github.com/pressly/goose) (migrations), [pgx](https://github.com/jackc/pgx) (Postgres driver)
- **Frontend:** Next.js 15 (App Router) · React 19 · TypeScript · Tailwind v4 · Lucide React
- **Banco:** PostgreSQL 16 (schemas `app` e `audit` com roles separados)
- **Cache/Sessões:** Redis 7
- **Fontes:** JetBrains Mono · Frank Ruhl Libre

## Rodando

Você não precisa instalar Go, Node, Postgres ou Redis localmente — só **Docker** e **Docker Compose v2**.

```bash
cp .env.example .env       # ajuste APP_ENV e portas se quiser
./tevunah setup            # up + migrate + seed do admin de dev
```

- Frontend: <http://localhost:3000>
- Backend:  <http://localhost:8080/api/health>

Em rodadas posteriores basta `./tevunah up`.

## Arquitetura de segurança (resumo)

Premissas (ver `tevunah/security`):

1. **Cadastro fechado.** Nenhuma rota pública cria usuários — apenas admin via app ou via `./tevunah admin:create`.
2. **ABAC + compartimentação.** Acesso = `papel.permissão` ∧ `clearance` ≥ `recurso.classificação` ∧ compartimentos compatíveis. Matriz `(role, action)` é **dado editável pelo admin** (tabela `app.permissions`).
3. **4-eyes parametrizável.** Ações sensíveis (atribuir papel/clearance, desativar, resetar TOTP) ficam em `app.pending_approvals` até serem confirmadas pelo papel `approver_role`.
4. **Audit append-only com cadeia de hash.** Schema `audit` com triggers que abortam UPDATE/DELETE/TRUNCATE; cada linha grava `prev_hash` e `hash = sha256(prev_hash || payload)`. Adulteração quebra a cadeia.
5. **Roles Postgres separados.** Aplicação conecta como `tevunah_app` (CRUD em `app.*`, SELECT/INSERT em `audit.audit_log`). Inserts no audit usam o role `tevunah_audit_writer` (apenas INSERT). Migrations rodam como superuser e ficam offline em produção.
6. **Sem hard delete.** Toda "exclusão" é soft delete (`deleted_at`).

## CLI `./tevunah`

Similar ao Laravel Sail. Lista completa com `./tevunah help`. Resumo:

```bash
# Lifecycle
./tevunah up                       # sobe (detached)
./tevunah up:fg                    # sobe no foreground
./tevunah down                     # derruba
./tevunah restart [svc]
./tevunah build [svc]
./tevunah rebuild                  # down + build --no-cache + up
./tevunah ps / status / logs [svc]

# Bootstrap e banco
./tevunah setup                    # up + migrate + seed dev
./tevunah migrate [up|down|status|version|redo|reset]
./tevunah admin:create             # admin novo (interativo) — PRODUÇÃO
./tevunah seed:dev                 # admin dev (idempotente; só se APP_ENV=development)

# Exec
./tevunah go test ./...
./tevunah npm install <pkg>
./tevunah npx <cmd>
./tevunah psql [...args]
./tevunah redis-cli [...args]
./tevunah sh:backend  /  sh:frontend  /  sh:postgres  /  sh:redis

# Limpeza
./tevunah prune                    # remove volumes
./tevunah nuke                     # remove volumes + imagens locais
```

## Banco — schemas

- **`app`** — domínio editável pela aplicação:
  - `roles` — papéis fixos (agente, analista, gestor, administrador) — *seed*.
  - `users` — agentes/usuários (`code`, `email`, `password_hash` Argon2id, `totp_secret`, `clearance_level`, `status`, `deleted_at`).
  - `user_roles` — vínculo n:n (multi-role).
  - `permissions` — matriz `(role_code, action, allowed, requires_dual_approval, approver_role)` — *seed* com o MVP.
  - `pending_approvals` — fila do 4-eyes (`expires_at` default = 48h).
- **`audit`** — append-only, cadeia de hash:
  - `chain_head` — uma linha; `last_hash` serializa inserts via `FOR UPDATE`.
  - `audit_log` — `id`, `ts`, `actor_*`, `action`, `resource_*`, `before`, `after`, `reason`, `prev_hash`, `hash`. Triggers bloqueiam UPDATE/DELETE/TRUNCATE.

Migrations em [backend/db/migrations/](backend/db/migrations/). Arquivos SQL são embutidos no binário via `embed.FS` — `./tevunah migrate up` aplica tudo.

## Bootstrap em produção

```bash
APP_ENV=production ./tevunah up
./tevunah migrate up
./tevunah admin:create        # prompts: email, nome, senha, TOTP secret é gerado e mostrado
```

O `seed:dev` é **inerte** quando `APP_ENV != development` — não há jeito acidental de criar admin com senha padrão em produção.

## Layout

Estética **terminal/tactical** — fundo escuro com scanlines CRT sutis, monospace, faixas de classificação `SAI 2º BPRAIO // TEVUNAH`, sidebar com os módulos do MVP: **Dashboard**, **Agentes** (CRUD de usuários), **Auditoria** (consulta do log para gestor e admin), **Admin** (configurações + matriz RBAC).

### Paletas

CSS variables em [frontend/app/globals.css](frontend/app/globals.css) expostas ao Tailwind via `@theme inline`. 4 paletas trocáveis via `data-palette` no `<html>`: `phosphor` (padrão), `amber`, `cyan`, `alert`. Toggle exposto pelo ícone ⚙ na topbar.

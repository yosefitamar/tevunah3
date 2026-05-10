-- +goose Up
-- +goose StatementBegin

CREATE TABLE app.roles (
  codename   text PRIMARY KEY,
  label      text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO app.roles (codename, label) VALUES
  ('agente',         'Agente'),
  ('analista',       'Analista'),
  ('gestor',         'Gestor'),
  ('administrador',  'Administrador');

CREATE TABLE app.users (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code            text NOT NULL UNIQUE,                    -- ex.: ANL-0042
  email           text NOT NULL UNIQUE,
  display_name    text NOT NULL,
  password_hash   text NOT NULL,                           -- Argon2id encoded
  totp_secret     text NULL,                               -- base32; cifrar app-side em prod
  totp_enrolled_at timestamptz NULL,
  clearance_level smallint NOT NULL DEFAULT 1
                  CHECK (clearance_level BETWEEN 1 AND 5),
  status          text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','suspended','deactivated')),
  last_login_at   timestamptz NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  created_by      uuid NULL REFERENCES app.users(id),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz NULL                          -- soft-delete
);

CREATE INDEX users_status_idx ON app.users (status) WHERE deleted_at IS NULL;
CREATE INDEX users_email_lower_idx ON app.users (lower(email));

CREATE TABLE app.user_roles (
  user_id      uuid NOT NULL REFERENCES app.users(id),
  role_code    text NOT NULL REFERENCES app.roles(codename),
  assigned_by  uuid NULL REFERENCES app.users(id),
  assigned_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_code)
);

CREATE TABLE app.permissions (
  role_code              text NOT NULL REFERENCES app.roles(codename),
  action                 text NOT NULL,
  allowed                boolean NOT NULL DEFAULT false,
  requires_dual_approval boolean NOT NULL DEFAULT false,
  approver_role          text NULL REFERENCES app.roles(codename),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  updated_by             uuid NULL REFERENCES app.users(id),
  PRIMARY KEY (role_code, action)
);

-- Matriz padrão para o MVP (apenas o CRUD de agentes + audit).
-- Admin tem tudo; algumas ações exigem 4-eyes com aprovação do gestor.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  -- Listagem e leitura
  ('gestor',        'user.list',           true,  false, NULL),
  ('administrador', 'user.list',           true,  false, NULL),
  ('agente',        'user.read.self',      true,  false, NULL),
  ('analista',      'user.read.self',      true,  false, NULL),
  ('gestor',        'user.read.self',      true,  false, NULL),
  ('administrador', 'user.read.self',      true,  false, NULL),
  ('agente',        'user.update.self',    true,  false, NULL),
  ('analista',      'user.update.self',    true,  false, NULL),
  ('gestor',        'user.update.self',    true,  false, NULL),
  ('administrador', 'user.update.self',    true,  false, NULL),

  -- Criação / desativação / atribuição de papel e clearance — admin com 4-eyes
  ('administrador', 'user.create',         true,  false, NULL),
  ('administrador', 'user.role.assign',    true,  true,  'gestor'),
  ('administrador', 'user.clearance.set',  true,  true,  'gestor'),
  ('administrador', 'user.deactivate',     true,  true,  'gestor'),

  -- Reset de credenciais
  ('administrador', 'user.password.reset', true,  false, NULL),
  ('administrador', 'user.totp.reset',     true,  true,  'gestor'),

  -- Audit log
  ('gestor',        'audit.read',          true,  false, NULL),
  ('administrador', 'audit.read',          true,  false, NULL);

CREATE TABLE app.pending_approvals (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action            text NOT NULL,
  resource_type     text NULL,
  resource_id       text NULL,
  payload           jsonb NOT NULL DEFAULT '{}'::jsonb,
  requested_by      uuid NOT NULL REFERENCES app.users(id),
  requested_at      timestamptz NOT NULL DEFAULT now(),
  required_approver_role text NOT NULL REFERENCES app.roles(codename),
  status            text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','approved','rejected','expired','cancelled')),
  decided_by        uuid NULL REFERENCES app.users(id),
  decided_at        timestamptz NULL,
  decision_reason   text NULL,
  expires_at        timestamptz NOT NULL DEFAULT (now() + interval '48 hours')
);

CREATE INDEX pending_approvals_status_idx ON app.pending_approvals (status, expires_at);

-- Permissões para tevunah_app no schema app
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA app TO tevunah_app;
-- DELETE permanece negado mesmo no schema app — só soft delete via deleted_at.
ALTER DEFAULT PRIVILEGES IN SCHEMA app GRANT SELECT, INSERT, UPDATE ON TABLES TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.pending_approvals;
DROP TABLE IF EXISTS app.permissions;
DROP TABLE IF EXISTS app.user_roles;
DROP TABLE IF EXISTS app.users;
DROP TABLE IF EXISTS app.roles;
-- +goose StatementEnd

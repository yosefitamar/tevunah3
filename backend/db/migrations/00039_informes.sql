-- +goose Up
-- +goose StatementBegin

-- Informes: captura rápida de algo de que o agente tomou ciência, que depois
-- subsidia relatórios detalhados. Campos: quando (data), onde, como, descrição
-- e foto opcional. Visibilidade é pool compartilhado gateado por clearance
-- (required_clearance ≤ clearance do agente; autor e admin sempre veem).
CREATE TABLE app.informes (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurred_on         date NOT NULL,                 -- QUANDO (só data)
  location            text NOT NULL DEFAULT '',      -- ONDE
  how                 text NOT NULL DEFAULT '',      -- COMO
  description         text NOT NULL DEFAULT '',      -- DESCRIÇÃO
  photo_path          text NULL,                     -- foto opcional (filename em PHOTO_DIR)
  required_clearance  smallint NOT NULL DEFAULT 1
                        CHECK (required_clearance BETWEEN 1 AND 5),
  version             integer NOT NULL DEFAULT 1,
  created_at          timestamptz NOT NULL DEFAULT now(),
  created_by          uuid NOT NULL REFERENCES app.users(id),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  updated_by          uuid NOT NULL REFERENCES app.users(id),
  deleted_at          timestamptz NULL,
  deleted_by          uuid NULL REFERENCES app.users(id)
);

CREATE INDEX informes_created_at_idx  ON app.informes (created_at DESC)  WHERE deleted_at IS NULL;
CREATE INDEX informes_occurred_on_idx ON app.informes (occurred_on DESC) WHERE deleted_at IS NULL;
CREATE INDEX informes_clearance_idx   ON app.informes (required_clearance) WHERE deleted_at IS NULL;

-- Permissões RBAC. Os 4 papéis recebem todas as ações; a POSSE (editar/excluir
-- só o próprio, salvo gestor/admin) é refinada no handler — a matriz é coarse.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  ('agente',        'informe.read',   true, false, NULL),
  ('analista',      'informe.read',   true, false, NULL),
  ('gestor',        'informe.read',   true, false, NULL),
  ('administrador', 'informe.read',   true, false, NULL),

  ('agente',        'informe.create', true, false, NULL),
  ('analista',      'informe.create', true, false, NULL),
  ('gestor',        'informe.create', true, false, NULL),
  ('administrador', 'informe.create', true, false, NULL),

  ('agente',        'informe.update', true, false, NULL),
  ('analista',      'informe.update', true, false, NULL),
  ('gestor',        'informe.update', true, false, NULL),
  ('administrador', 'informe.update', true, false, NULL),

  ('agente',        'informe.delete', true, false, NULL),
  ('analista',      'informe.delete', true, false, NULL),
  ('gestor',        'informe.delete', true, false, NULL),
  ('administrador', 'informe.delete', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions WHERE action IN ('informe.read','informe.create','informe.update','informe.delete');
DROP TABLE IF EXISTS app.informes;
-- +goose StatementEnd

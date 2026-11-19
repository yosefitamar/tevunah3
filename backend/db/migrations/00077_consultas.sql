-- +goose Up
-- +goose StatementBegin

-- Registro de consultas: o agente/analista anota cada pesquisa que fez em
-- sistemas externos a pedido de alguém — quando, em qual sistema, o que foi
-- pesquisado, quem pediu, por quê e o que encontrou. Serve de justificativa
-- quando o sistema consultado é auditado.
--
-- Visibilidade: cada um vê só os próprios registros; quem tem a ação
-- consulta.read_all (gestor e administrador, por padrão) vê os de todos.
-- Edição e exclusão (lógica) ficam na trilha de auditoria, com antes e depois.
CREATE TABLE app.consultas (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  performed_at   timestamptz NOT NULL,            -- DATA e HORA da consulta
  system         text NOT NULL,                   -- SISTEMA consultado (caixa alta)
  target         text NOT NULL,                   -- ALVO: nome, CPF, placa, telefone…
  requester      text NOT NULL,                   -- SOLICITANTE
  requester_org  text NOT NULL DEFAULT '',        -- ÓRGÃO/unidade do solicitante
  reference_doc  text NOT NULL DEFAULT '',        -- ofício, protocolo, BO, procedimento
  reason         text NOT NULL,                   -- MOTIVO
  result         text NOT NULL,                   -- RESULTADO
  version        integer NOT NULL DEFAULT 1,
  created_at     timestamptz NOT NULL DEFAULT now(),
  created_by     uuid NOT NULL REFERENCES app.users(id),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  updated_by     uuid NOT NULL REFERENCES app.users(id),
  deleted_at     timestamptz NULL,
  deleted_by     uuid NULL REFERENCES app.users(id)
);

CREATE INDEX consultas_performed_at_idx ON app.consultas (performed_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX consultas_created_by_idx   ON app.consultas (created_by, performed_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX consultas_system_idx       ON app.consultas (system) WHERE deleted_at IS NULL;

-- Permissões RBAC. A POSSE (editar/excluir só o próprio, salvo gestor/admin) é
-- refinada no handler; consulta.read_all abre os registros dos demais agentes.
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  ('agente',        'consulta.read',     true, false, NULL),
  ('analista',      'consulta.read',     true, false, NULL),
  ('gestor',        'consulta.read',     true, false, NULL),
  ('administrador', 'consulta.read',     true, false, NULL),

  ('agente',        'consulta.create',   true, false, NULL),
  ('analista',      'consulta.create',   true, false, NULL),
  ('gestor',        'consulta.create',   true, false, NULL),
  ('administrador', 'consulta.create',   true, false, NULL),

  ('agente',        'consulta.update',   true, false, NULL),
  ('analista',      'consulta.update',   true, false, NULL),
  ('gestor',        'consulta.update',   true, false, NULL),
  ('administrador', 'consulta.update',   true, false, NULL),

  ('agente',        'consulta.delete',   true, false, NULL),
  ('analista',      'consulta.delete',   true, false, NULL),
  ('gestor',        'consulta.delete',   true, false, NULL),
  ('administrador', 'consulta.delete',   true, false, NULL),

  ('gestor',        'consulta.read_all', true, false, NULL),
  ('administrador', 'consulta.read_all', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN ('consulta.read','consulta.create','consulta.update','consulta.delete','consulta.read_all');
DROP TABLE IF EXISTS app.consultas;
-- +goose StatementEnd

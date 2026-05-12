-- +goose Up
-- +goose StatementBegin

-- Actions do módulo Entidades. Sem 4-eyes (decisão de design: entidades
-- são CRUD direto, diferente de agentes). Controle por classification fica
-- na lógica do app (clearance_level >= classification).
INSERT INTO app.permissions (role_code, action, allowed, requires_dual_approval, approver_role) VALUES
  -- Listagem e leitura: todos os papéis logados (clearance filtra na leitura)
  ('agente',        'entity.list',   true, false, NULL),
  ('analista',      'entity.list',   true, false, NULL),
  ('gestor',        'entity.list',   true, false, NULL),
  ('administrador', 'entity.list',   true, false, NULL),

  ('agente',        'entity.read',   true, false, NULL),
  ('analista',      'entity.read',   true, false, NULL),
  ('gestor',        'entity.read',   true, false, NULL),
  ('administrador', 'entity.read',   true, false, NULL),

  -- Criação e edição: analista produz inteligência; gestor/admin também.
  -- Agente não cria nem edita por ora (consumidor de inteligência, não produtor).
  ('analista',      'entity.create', true, false, NULL),
  ('gestor',        'entity.create', true, false, NULL),
  ('administrador', 'entity.create', true, false, NULL),

  ('analista',      'entity.update', true, false, NULL),
  ('gestor',        'entity.update', true, false, NULL),
  ('administrador', 'entity.update', true, false, NULL),

  -- Exclusão lógica (soft): só gestor e administrador.
  ('gestor',        'entity.delete', true, false, NULL),
  ('administrador', 'entity.delete', true, false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN (
   'entity.list',
   'entity.read',
   'entity.create',
   'entity.update',
   'entity.delete'
 );
-- +goose StatementEnd

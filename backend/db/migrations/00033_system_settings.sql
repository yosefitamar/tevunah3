-- +goose Up
-- +goose StatementBegin

-- Tabela singleton de configurações do sistema. Modelagem:
--   - Uma única linha, identificada por `key = 'singleton'`.
--   - Constraint UNIQUE no key (mais um CHECK forçando o valor) garante que
--     nunca exista mais de uma linha — qualquer INSERT que tente duplicar
--     gera erro. Update sempre via UPDATE, nunca INSERT.
--
-- Campos:
--   agency_name     — nome curto exibido no cabeçalho/rodapé da UI ("SAI 2º BPRAIO")
--   document_title  — texto usado como default do campo ORIGEM em RIs novos
--                     ("CCINT/ASINT/PMCE")
--   brasao_path     — filename relativo a PHOTO_DIR do brasão usado no PDF.
--                     Quando vazio, o PDF cai no asset estático (logo-sai.png)
--                     já presente no PHOTO_DIR (compatibilidade pré-config).
CREATE TABLE app.system_settings (
  key             text NOT NULL PRIMARY KEY
                  CHECK (key = 'singleton'),
  agency_name     text NOT NULL DEFAULT '',
  document_title  text NOT NULL DEFAULT '',
  brasao_path     text NULL,
  updated_at      timestamptz NOT NULL DEFAULT now(),
  updated_by      uuid NULL REFERENCES app.users(id)
);

-- Seed inicial preservando o que era hardcoded no front (SAI 2º BPRAIO)
-- e o default antigo do origin (CCINT/ASINT/PMCE) — operacionalmente o
-- sistema continua igual ao primeiro deploy desta migration.
INSERT INTO app.system_settings (key, agency_name, document_title)
VALUES ('singleton', 'SAI 2º BPRAIO', 'CCINT/ASINT/PMCE');

-- Permissões:
--   read:   qualquer usuário autenticado (todo papel precisa renderizar o
--           nome da agência no shell, e o valor de origin no form de novo RI)
--   update: apenas administrador
INSERT INTO app.permissions
  (role_code, action, allowed, requires_dual_approval, approver_role)
VALUES
  ('agente',        'system.settings.read',   true,  false, NULL),
  ('analista',      'system.settings.read',   true,  false, NULL),
  ('gestor',        'system.settings.read',   true,  false, NULL),
  ('administrador', 'system.settings.read',   true,  false, NULL),
  ('administrador', 'system.settings.update', true,  false, NULL);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM app.permissions
 WHERE action IN ('system.settings.read', 'system.settings.update');
DROP TABLE IF EXISTS app.system_settings;
-- +goose StatementEnd

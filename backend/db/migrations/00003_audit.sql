-- +goose Up
-- +goose StatementBegin

-- Single-row table que armazena o hash do último registro do audit_log.
-- Serializa os inserts via row lock (FOR UPDATE), garantindo a cadeia.
CREATE TABLE audit.chain_head (
  pk          smallint PRIMARY KEY DEFAULT 1 CHECK (pk = 1),
  last_hash   bytea    NOT NULL,
  last_id     bigint   NOT NULL DEFAULT 0,
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- Hash de gênese: sha256("TEVUNAH:AUDIT:GENESIS")
INSERT INTO audit.chain_head (pk, last_hash, last_id)
VALUES (1, digest('TEVUNAH:AUDIT:GENESIS', 'sha256'), 0);

CREATE TABLE audit.audit_log (
  id                       bigint       PRIMARY KEY,
  ts                       timestamptz  NOT NULL DEFAULT now(),
  actor_user_id            uuid         NULL,   -- pode ser NULL em LOGIN_DENIED com email inexistente
  actor_session_id         text         NULL,
  actor_ip                 inet         NULL,
  actor_terminal           text         NULL,
  action                   text         NOT NULL,
  resource_type            text         NULL,
  resource_id              text         NULL,
  resource_classification  smallint     NULL,
  before                   jsonb        NULL,
  after                    jsonb        NULL,
  reason                   text         NULL,
  prev_hash                bytea        NOT NULL,
  hash                     bytea        NOT NULL
);

CREATE INDEX audit_log_ts_idx          ON audit.audit_log (ts DESC);
CREATE INDEX audit_log_actor_idx       ON audit.audit_log (actor_user_id, ts DESC);
CREATE INDEX audit_log_action_idx      ON audit.audit_log (action, ts DESC);
CREATE INDEX audit_log_resource_idx    ON audit.audit_log (resource_type, resource_id, ts DESC);

-- BEFORE INSERT: calcula id, prev_hash, hash, e atualiza chain_head.
CREATE OR REPLACE FUNCTION audit.tg_audit_log_chain() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  v_prev_hash bytea;
  v_last_id   bigint;
  v_payload   bytea;
BEGIN
  -- Lock da cabeça da cadeia: serializa inserts concorrentes.
  SELECT last_hash, last_id
    INTO v_prev_hash, v_last_id
    FROM audit.chain_head
   WHERE pk = 1
   FOR UPDATE;

  NEW.id := v_last_id + 1;
  NEW.prev_hash := v_prev_hash;

  -- Payload canônico: concatena campos relevantes. Ordem fixa.
  v_payload := convert_to(
    coalesce(NEW.id::text,'')||'|'||
    coalesce(NEW.ts::text,'')||'|'||
    coalesce(NEW.actor_user_id::text,'')||'|'||
    coalesce(NEW.actor_session_id,'')||'|'||
    coalesce(host(NEW.actor_ip),'')||'|'||
    coalesce(NEW.actor_terminal,'')||'|'||
    NEW.action||'|'||
    coalesce(NEW.resource_type,'')||'|'||
    coalesce(NEW.resource_id,'')||'|'||
    coalesce(NEW.resource_classification::text,'')||'|'||
    coalesce(NEW.before::text,'')||'|'||
    coalesce(NEW.after::text,'')||'|'||
    coalesce(NEW.reason,''),
    'UTF8'
  );

  NEW.hash := digest(v_prev_hash || v_payload, 'sha256');

  UPDATE audit.chain_head
     SET last_hash = NEW.hash,
         last_id   = NEW.id,
         updated_at = now()
   WHERE pk = 1;

  RETURN NEW;
END;
$$;

CREATE TRIGGER audit_log_chain
  BEFORE INSERT ON audit.audit_log
  FOR EACH ROW EXECUTE FUNCTION audit.tg_audit_log_chain();

-- Bloqueia UPDATE e DELETE — append-only de verdade, independente do papel.
CREATE OR REPLACE FUNCTION audit.tg_audit_log_block_modify() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit.audit_log é append-only — operação % proibida', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_log_block_update
  BEFORE UPDATE ON audit.audit_log
  FOR EACH ROW EXECUTE FUNCTION audit.tg_audit_log_block_modify();

CREATE TRIGGER audit_log_block_delete
  BEFORE DELETE ON audit.audit_log
  FOR EACH ROW EXECUTE FUNCTION audit.tg_audit_log_block_modify();

-- Bloqueia TRUNCATE no nível da tabela.
CREATE TRIGGER audit_log_block_truncate
  BEFORE TRUNCATE ON audit.audit_log
  FOR EACH STATEMENT EXECUTE FUNCTION audit.tg_audit_log_block_modify();

-- Grants:
-- tevunah_audit_writer: apenas INSERT (e SELECT da chain_head é necessário pelo trigger,
--   mas o trigger roda como dono da função — então o writer só precisa de INSERT).
-- tevunah_app: SELECT (para os módulos de auditoria do gestor/admin) + INSERT.
GRANT INSERT ON audit.audit_log TO tevunah_audit_writer;
GRANT SELECT, INSERT ON audit.audit_log TO tevunah_app;
GRANT SELECT ON audit.chain_head TO tevunah_app;

-- A função roda SECURITY DEFINER para conseguir tocar em chain_head com privilégios do owner.
ALTER FUNCTION audit.tg_audit_log_chain() SECURITY DEFINER;
ALTER FUNCTION audit.tg_audit_log_block_modify() SECURITY DEFINER;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit.audit_log;
DROP FUNCTION IF EXISTS audit.tg_audit_log_chain();
DROP FUNCTION IF EXISTS audit.tg_audit_log_block_modify();
DROP TABLE IF EXISTS audit.chain_head;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- Adiciona actor_user_agent ao trilho de auditoria.
--
-- Cuidado com a cadeia de hash: linhas pré-existentes foram calculadas com o
-- algoritmo v1 (sem este campo). Suas linhas continuam válidas e a cadeia
-- segue conectada (prev_hash da próxima entrada continua sendo o hash da
-- última entrada antiga). A nova função v2 inclui actor_user_agent no payload
-- canônico — entradas pós-migration usam v2.
--
-- Para um futuro endpoint /api/audit/verify, será preciso saber qual algoritmo
-- aplicar a cada faixa de ids. Por ora não há verify, então sem impacto prático.

ALTER TABLE audit.audit_log ADD COLUMN actor_user_agent text NULL;

CREATE OR REPLACE FUNCTION audit.tg_audit_log_chain() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  v_prev_hash bytea;
  v_last_id   bigint;
  v_payload   bytea;
BEGIN
  SELECT last_hash, last_id
    INTO v_prev_hash, v_last_id
    FROM audit.chain_head
   WHERE pk = 1
   FOR UPDATE;

  NEW.id := v_last_id + 1;
  NEW.prev_hash := v_prev_hash;

  -- Payload canônico v2: actor_user_agent posicionado após actor_terminal.
  v_payload := convert_to(
    coalesce(NEW.id::text,'')||'|'||
    coalesce(NEW.ts::text,'')||'|'||
    coalesce(NEW.actor_user_id::text,'')||'|'||
    coalesce(NEW.actor_session_id,'')||'|'||
    coalesce(host(NEW.actor_ip),'')||'|'||
    coalesce(NEW.actor_terminal,'')||'|'||
    coalesce(NEW.actor_user_agent,'')||'|'||
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

ALTER FUNCTION audit.tg_audit_log_chain() SECURITY DEFINER;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Restaura função sem o campo (v1).
CREATE OR REPLACE FUNCTION audit.tg_audit_log_chain() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  v_prev_hash bytea;
  v_last_id   bigint;
  v_payload   bytea;
BEGIN
  SELECT last_hash, last_id INTO v_prev_hash, v_last_id
    FROM audit.chain_head WHERE pk = 1 FOR UPDATE;

  NEW.id := v_last_id + 1;
  NEW.prev_hash := v_prev_hash;

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

  UPDATE audit.chain_head SET last_hash = NEW.hash, last_id = NEW.id, updated_at = now()
   WHERE pk = 1;

  RETURN NEW;
END;
$$;
ALTER FUNCTION audit.tg_audit_log_chain() SECURITY DEFINER;

ALTER TABLE audit.audit_log DROP COLUMN IF EXISTS actor_user_agent;

-- +goose StatementEnd

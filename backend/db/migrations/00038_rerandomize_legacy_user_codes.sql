-- +goose Up
-- +goose StatementBegin

-- O importador antigo gerava códigos sequenciais 'AGT-NNNN', mas o padrão do
-- sistema é 4 dígitos aleatórios (users.GenerateCode). Re-sorteia um código
-- único de 4 dígitos para cada usuário cujo code ainda esteja no formato
-- AGT-NNNN. Não toca o sentinela 'SYS-IMPORT' nem usuários normais (já 4
-- dígitos). No-op em ambientes sem usuários importados.
DO $$
DECLARE
  u        RECORD;
  newcode  text;
  tries    int;
BEGIN
  FOR u IN SELECT id FROM app.users WHERE code ~ '^AGT-[0-9]+$' LOOP
    tries := 0;
    LOOP
      newcode := lpad((floor(random() * 10000))::int::text, 4, '0');
      EXIT WHEN NOT EXISTS (SELECT 1 FROM app.users WHERE code = newcode);
      tries := tries + 1;
      IF tries > 100 THEN
        RAISE EXCEPTION 'não foi possível gerar código único para usuário %', u.id;
      END IF;
    END LOOP;
    UPDATE app.users SET code = newcode, updated_at = now() WHERE id = u.id;
  END LOOP;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Irreversível: os códigos AGT-NNNN originais foram descartados.
SELECT 1;
-- +goose StatementEnd

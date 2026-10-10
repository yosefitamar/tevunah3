-- +goose Up
-- +goose StatementBegin

-- ─── Referência de área da unidade militar ────────────────────────────
-- A área (companhia territorial do local do fato) sai do catálogo do SIPOM
-- quando ele tem regra para o bairro ou a cidade. Quando não tem — ou tem
-- mais de uma —, a ocorrência fica pendente e o analista escolhe.
--
-- Cada escolha do analista vira referência aqui: cidade + bairro → área. Nas
-- próximas ocorrências do mesmo lugar a área já entra sozinha, e a referência
-- vale sobre o catálogo (foi um analista que decidiu). Escolher outra área
-- para o mesmo lugar substitui a referência.
--
-- city_key/neighborhood_key são a forma de comparação (sem acento, caixa nem
-- pontuação — a mesma do catálogo, calculada em internal/sipom); city e
-- neighborhood guardam a grafia do relatório, para exibição. Bairro vazio é
-- uma referência própria: só vale para ocorrência sem bairro naquela cidade.
CREATE TABLE app.sipom_area_rules (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  city             text NOT NULL,
  neighborhood     text NOT NULL DEFAULT '',
  city_key         text NOT NULL,
  neighborhood_key text NOT NULL DEFAULT '',
  area_id          integer NOT NULL REFERENCES sipom.companhias(id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  created_by       uuid NOT NULL REFERENCES app.users(id),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  updated_by       uuid NOT NULL REFERENCES app.users(id),
  CONSTRAINT sipom_area_rules_place_uniq UNIQUE (city_key, neighborhood_key)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS app.sipom_area_rules;
-- +goose StatementEnd

-- +goose Up
-- +goose StatementBegin

-- ─── De-para de naturezas: confiança pela evidência da ficha ──────────
-- O relatório do CPRAIO muitas vezes descreve a ação ("APRESENTAÇÃO E
-- APREENSÃO DE ARMA DE FOGO"), não o crime. Até aqui essas regras eram
-- 'sugerida': a natureza entrava, mas toda ocorrência ficava pendente de
-- confirmação — mesmo quando a própria ficha trazia a arma apreendida.
--
-- Dois níveis novos, entre a direta e a sugerida:
--   'com_arma'  — aplica sozinha quando a ficha traz arma de fogo na tabela
--                 de armas apreendidas; sem a arma, vale como sugerida.
--   'com_droga' — idem, para a tabela de drogas apreendidas.
ALTER TABLE app.sipom_natureza_map
  DROP CONSTRAINT sipom_natureza_map_confianca_check,
  ADD CONSTRAINT sipom_natureza_map_confianca_check
    CHECK (confianca IN ('direta', 'sugerida', 'com_arma', 'com_droga'));

-- Apreensão de arma com arma de fogo na ficha deixa de pedir confirmação.
-- Só troca a regra que ainda está como veio na carga inicial.
UPDATE app.sipom_natureza_map
   SET confianca = 'com_arma', updated_at = now()
 WHERE app.norm_txt(source) = app.norm_txt('APRESENTAÇÃO E APREENSÃO DE ARMA DE FOGO')
   AND confianca = 'sugerida';

-- O mesmo para a apreensão de drogas com droga na ficha: o relatório usa
-- essa natureza quando a droga é achada sem autor (o consumo tem natureza
-- própria, "POSSE DE DROGA PARA CONSUMO"), e no SIPOM isso é tráfico.
UPDATE app.sipom_natureza_map
   SET confianca = 'com_droga', updated_at = now()
 WHERE app.norm_txt(source) = app.norm_txt('APRESENTAÇÃO E APREENSÃO DE DROGAS')
   AND confianca = 'sugerida';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE app.sipom_natureza_map SET confianca = 'sugerida' WHERE confianca IN ('com_arma', 'com_droga');
ALTER TABLE app.sipom_natureza_map
  DROP CONSTRAINT sipom_natureza_map_confianca_check,
  ADD CONSTRAINT sipom_natureza_map_confianca_check CHECK (confianca IN ('direta', 'sugerida'));
-- +goose StatementEnd

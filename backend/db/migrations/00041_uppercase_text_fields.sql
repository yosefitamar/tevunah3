-- +goose Up
-- +goose StatementBegin

-- Normaliza para MAIÚSCULAS os campos textuais já gravados. O formulário
-- sempre exibiu uppercase via CSS, mas o valor era persistido como digitado;
-- a partir desta versão o backend normaliza na escrita (upperTrim) e esta
-- migration alinha o legado. Campos com semântica própria ficam fora:
-- gender/category (enums lowercase), cpf/tax_id/renavam/cep (dígitos),
-- tags (lowercase por design), plate (já normalizada).

UPDATE app.entities
   SET name        = upper(name),
       description = upper(description)
 WHERE name IS DISTINCT FROM upper(name)
    OR description IS DISTINCT FROM upper(description);

UPDATE app.entity_persons
   SET aliases     = (SELECT COALESCE(array_agg(upper(btrim(a)) ORDER BY ord), '{}')
                        FROM unnest(aliases) WITH ORDINALITY AS t(a, ord)),
       mother_name = upper(mother_name)
 WHERE mother_name IS DISTINCT FROM upper(mother_name)
    OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE a IS DISTINCT FROM upper(btrim(a)));

UPDATE app.entity_organizations
   SET aliases    = (SELECT COALESCE(array_agg(upper(btrim(a)) ORDER BY ord), '{}')
                       FROM unnest(aliases) WITH ORDINALITY AS t(a, ord)),
       legal_name = upper(legal_name)
 WHERE legal_name IS DISTINCT FROM upper(legal_name)
    OR EXISTS (SELECT 1 FROM unnest(aliases) a WHERE a IS DISTINCT FROM upper(btrim(a)));

UPDATE app.entity_places
   SET address = upper(address),
       country = upper(country),
       region  = upper(region)
 WHERE address IS DISTINCT FROM upper(address)
    OR country IS DISTINCT FROM upper(country)
    OR region IS DISTINCT FROM upper(region);

UPDATE app.entity_vehicles
   SET brand   = upper(brand),
       model   = upper(model),
       color   = upper(color),
       chassis = upper(chassis)
 WHERE brand IS DISTINCT FROM upper(brand)
    OR model IS DISTINCT FROM upper(model)
    OR color IS DISTINCT FROM upper(color)
    OR chassis IS DISTINCT FROM upper(chassis);

UPDATE app.person_addresses
   SET street       = upper(street),
       number       = upper(number),
       complement   = upper(complement),
       neighborhood = upper(neighborhood),
       city         = upper(city),
       state        = upper(state)
 WHERE street IS DISTINCT FROM upper(street)
    OR number IS DISTINCT FROM upper(number)
    OR complement IS DISTINCT FROM upper(complement)
    OR neighborhood IS DISTINCT FROM upper(neighborhood)
    OR city IS DISTINCT FROM upper(city)
    OR state IS DISTINCT FROM upper(state);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Irreversível: a capitalização original não é preservada.
SELECT 1;
-- +goose StatementEnd

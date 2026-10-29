-- +goose Up
-- +goose StatementBegin

-- O analista pode apagar uma referência de área errada na tela de
-- referências (a ocorrência volta ao catálogo ou à escolha manual).
GRANT DELETE ON app.sipom_area_rules TO tevunah_app;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
REVOKE DELETE ON app.sipom_area_rules FROM tevunah_app;
-- +goose StatementEnd

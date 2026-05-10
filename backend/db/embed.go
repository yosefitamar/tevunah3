// Package db expõe os arquivos SQL de migration via embed.FS para serem
// embarcados nos binários do projeto (cmd/migrate, cmd/admin).
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS

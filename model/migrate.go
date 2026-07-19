package model

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var FS embed.FS

// TableName is this module's dedicated goose version-tracking table.
const TableName = "goose_db_version_tags"

func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetTableName(TableName)
	return goose.UpContext(ctx, db, "migrations")
}

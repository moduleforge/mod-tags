module github.com/moduleforge/tags-api

go 1.26.2

require (
	github.com/go-chi/chi/v5 v5.2.5
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.1
	github.com/moduleforge/core-api v0.0.0
	github.com/moduleforge/core-model v0.0.0
	github.com/moduleforge/tags-model v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

// Local path replaces for non-workspace (Docker) builds. The top-level
// go.work has the same replaces for workspace builds; these handle the
// non-workspace case where only this go.mod is visible.
replace github.com/moduleforge/core-model v0.0.0 => ../../mod-core/model

replace github.com/moduleforge/core-api v0.0.0 => ../../mod-core/api

replace github.com/moduleforge/tags-model v0.0.0 => ../model

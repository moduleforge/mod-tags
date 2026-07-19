//go:build integration

package service

// tag_template_upsert_integration_test.go verifies, against the same live
// Postgres instance tag_grant_integration_test.go's TestMain already spins up
// (schema, pool, and Services are shared -- see that file's doc comment for
// the full harness), that TagTemplateService.Upsert's ON CONFLICT (scope,
// purpose, value) WHERE scope IS NOT NULL clause actually engages
// tag_templates_scoped_purpose_value_idx as its arbiter rather than erroring
// at query time. Static review cannot confirm this: Postgres requires an ON
// CONFLICT clause's predicate to match a partial index's predicate exactly,
// and a mismatch surfaces only as a runtime error, not at query-authoring or
// sqlc-compile time -- see model/queries/tag_templates.sql's UpsertTagTemplate
// doc comment.
//
// Run with (from mod-tags/api):
//
//	go test -tags=integration -p 1 -v ./service/... -run TestTagTemplateUpsertIntegration

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// seedApp inserts a real entities row (fundamental type 'app') plus its
// corresponding apps row, satisfying the apps_check_type trigger
// (0015_apps.sql) so tag_templates.scope's FK to apps(id) is genuinely
// exercised, not just simulated.
func seedApp(ctx context.Context, pool *pgxpool.Pool, slug, name string) (int64, error) {
	var appTypeID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM types WHERE slug = 'app'`).Scan(&appTypeID); err != nil {
		return 0, fmt.Errorf("seedApp(%s): resolve app type: %w", slug, err)
	}
	var entityID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO entities (fundamental_type_id) VALUES ($1) RETURNING id`, appTypeID,
	).Scan(&entityID); err != nil {
		return 0, fmt.Errorf("seedApp(%s): insert entity: %w", slug, err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO apps (id, slug, name) VALUES ($1, $2, $3)`, entityID, slug, name,
	); err != nil {
		return 0, fmt.Errorf("seedApp(%s): insert app: %w", slug, err)
	}
	return entityID, nil
}

func TestTagTemplateUpsertIntegration(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool
	coreQ := coredb.New(pool)
	tagQ := tagsdb.New(pool)

	appID, err := seedApp(ctx, pool, "tag-template-upsert-verify-app", "Upsert Verify App")
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	appUUID := mustEntityUUID(ctx, t, pool, appID)

	color1 := "#111111FF"
	first, err := integrationSvcs.TagTemplate.Upsert(ctx, coreQ, tagQ, UpsertTagTemplateInput{
		Scope:     appUUID,
		Purpose:   "template-verify",
		Value:     "active",
		Label:     "Active",
		Color:     &color1,
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if first.Label != "Active" || first.SortOrder != 1 {
		t.Fatalf("first: got %+v, want label=Active sortOrder=1", first)
	}

	// Second call, same (scope, purpose, value), different label/color/
	// sort_order -- this is the call that fails at runtime (not at
	// sqlc-generate or go-build time) if the ON CONFLICT predicate doesn't
	// match the partial index exactly.
	color2 := "#222222FF"
	second, err := integrationSvcs.TagTemplate.Upsert(ctx, coreQ, tagQ, UpsertTagTemplateInput{
		Scope:     appUUID,
		Purpose:   "template-verify",
		Value:     "active",
		Label:     "Active (renamed)",
		Color:     &color2,
		SortOrder: 2,
	})
	if err != nil {
		t.Fatalf("second Upsert (ON CONFLICT against partial index): %v", err)
	}
	if second.Label != "Active (renamed)" || second.SortOrder != 2 {
		t.Fatalf("second: got %+v, want label=%q sortOrder=2", second, "Active (renamed)")
	}
	if second.Color == nil || *second.Color != color2 {
		t.Fatalf("second.Color: got %v, want %q", second.Color, color2)
	}

	// Confirm exactly one row exists for this (scope, purpose, value) --
	// proof the second call updated in place rather than inserting a
	// duplicate (which a non-matching ON CONFLICT predicate would either
	// error on, or -- if some other constraint silently absorbed it --
	// produce two rows for).
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM tag_templates WHERE scope = $1 AND purpose = $2 AND value = $3`,
		appID, "template-verify", "active",
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count for (scope, purpose, value): got %d, want 1", count)
	}

	// Confirm the persisted row reflects the second call's values, not the
	// first's (i.e. a real UPDATE happened, not a second INSERT that some
	// other path happened to dedupe away).
	var label, dbColor string
	var sortOrder int32
	if err := pool.QueryRow(ctx,
		`SELECT label, color, sort_order FROM tag_templates WHERE scope = $1 AND purpose = $2 AND value = $3`,
		appID, "template-verify", "active",
	).Scan(&label, &dbColor, &sortOrder); err != nil {
		t.Fatalf("select persisted row: %v", err)
	}
	if label != "Active (renamed)" || dbColor != color2 || sortOrder != 2 {
		t.Fatalf("persisted row: got label=%q color=%q sortOrder=%d, want label=%q color=%q sortOrder=2",
			label, dbColor, sortOrder, "Active (renamed)", color2)
	}
}

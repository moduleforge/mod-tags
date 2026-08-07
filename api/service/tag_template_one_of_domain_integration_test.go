//go:build integration

package service

// tag_template_one_of_domain_integration_test.go proves against a live
// Postgres that the rewritten ListTagTemplates query (the LEFT JOIN
// tag_purpose_policies added alongside the pre-existing LEFT JOIN entities)
// actually returns the tag_purpose_policies flag: a seeded
// one_of_domain = true purpose reads back true, an explicit false reads back
// false, and a purpose with no policy row reads back false via the query's
// own COALESCE default -- the same default the DB trigger
// (model/migrations/0205_tags_one_of_domain.sql) applies. Unit tests
// (tag_template_test.go) already exercise the mock's simulation of the join;
// only this test exercises the real SQL.
//
// This file reuses tag_grant_integration_test.go's shared TestMain (in the
// same package, same build tag) for the ephemeral composed-migrations
// directory, connectivity probing, and integrationPool/integrationSvcs
// setup -- see that file for those details. It does not define its own
// TestMain.
//
// Deliberately creates no tags rows anywhere in this test: the whole point
// of the change under test is that the flag must be readable for a purpose
// that has catalog entries but no real tag yet. Do not "helpfully" add tag
// seeding here.
//
// Run with (from mod-tags/api):
//
//	go test -tags=integration -p 1 -v ./service/... -run TestTagTemplateCatalogOneOfDomainIntegration

import (
	"context"
	"testing"

	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// TestTagTemplateCatalogOneOfDomainIntegration seeds three purposes covering
// the three cases the ListTagTemplates COALESCE join must distinguish --
// explicit true, explicit false, and no policy row at all -- seeds catalog
// (tag_templates) rows for each purpose but never any tags row, then asserts
// TagTemplate.List reports the expected OneOfDomain value for every case.
func TestTagTemplateCatalogOneOfDomainIntegration(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool
	coreQ := coredb.New(pool)
	tagQ := tagsdb.New(pool)

	const (
		purposeTrue  = "catalog-one-of-domain-verify-true"
		purposeFalse = "catalog-one-of-domain-verify-false"
		purposeNone  = "catalog-one-of-domain-verify-none"
	)

	appID, err := seedApp(ctx, pool, "catalog-one-of-domain-verify-app", "Catalog One Of Domain Verify App")
	if err != nil {
		t.Fatalf("seed app: %v", err)
	}
	appUUID := mustEntityUUID(ctx, t, pool, appID)

	// Seed the policy registry: true for purposeTrue, explicit false for
	// purposeFalse. purposeNone is never passed to Upsert -- that absence is
	// the "no policy row" case itself.
	if _, err := integrationSvcs.TagPurposePolicy.Upsert(ctx, tagQ, purposeTrue, true); err != nil {
		t.Fatalf("seed tag_purpose_policies(%s, true): %v", purposeTrue, err)
	}
	if _, err := integrationSvcs.TagPurposePolicy.Upsert(ctx, tagQ, purposeFalse, false); err != nil {
		t.Fatalf("seed tag_purpose_policies(%s, false): %v", purposeFalse, err)
	}

	// Seed one app-scoped catalog row per purpose for the true/false cases
	// (exercising the scoped read path and, incidentally, the pre-existing
	// LEFT JOIN entities scope hydration). No tags rows are created anywhere
	// in this test -- see file-level doc comment.
	if _, err := integrationSvcs.TagTemplate.Upsert(ctx, coreQ, tagQ, UpsertTagTemplateInput{
		Scope: appUUID, Purpose: purposeTrue, Value: "active", Label: "Active", SortOrder: 1,
	}); err != nil {
		t.Fatalf("seed tag_templates(%s): %v", purposeTrue, err)
	}
	if _, err := integrationSvcs.TagTemplate.Upsert(ctx, coreQ, tagQ, UpsertTagTemplateInput{
		Scope: appUUID, Purpose: purposeFalse, Value: "active", Label: "Active", SortOrder: 1,
	}); err != nil {
		t.Fatalf("seed tag_templates(%s): %v", purposeFalse, err)
	}

	// A global (NULL-scope) row for purposeNone, covering the global read
	// path alongside the scoped path above (Requirement 5). TagTemplate.Upsert
	// deliberately rejects a nil scope, so this row is inserted directly.
	if _, err := pool.Exec(ctx,
		`INSERT INTO tag_templates (scope, purpose, value, label, sort_order) VALUES (NULL, $1, $2, $3, $4)`,
		purposeNone, "active", "Active", 1,
	); err != nil {
		t.Fatalf("seed global tag_templates(%s): %v", purposeNone, err)
	}

	t.Run("seeded_true", func(t *testing.T) {
		rows, err := integrationSvcs.TagTemplate.List(ctx, coreQ, tagQ, purposeTrue, &appUUID)
		if err != nil {
			t.Fatalf("List(%s): %v", purposeTrue, err)
		}
		if len(rows) == 0 {
			t.Fatalf("List(%s): got 0 rows, want at least 1", purposeTrue)
		}
		sawScope := false
		for _, r := range rows {
			if !r.OneOfDomain {
				t.Errorf("List(%s): row %+v has OneOfDomain = false, want true", purposeTrue, r)
			}
			if r.Scope != nil && *r.Scope == appUUID {
				sawScope = true
			}
		}
		// Requirement 5: the added tag_purpose_policies join must not disturb
		// the pre-existing LEFT JOIN entities scope hydration.
		if !sawScope {
			t.Errorf("List(%s): no row hydrated Scope == %s (the seeded app)", purposeTrue, appUUID)
		}
	})

	t.Run("explicit_false", func(t *testing.T) {
		rows, err := integrationSvcs.TagTemplate.List(ctx, coreQ, tagQ, purposeFalse, &appUUID)
		if err != nil {
			t.Fatalf("List(%s): %v", purposeFalse, err)
		}
		if len(rows) == 0 {
			t.Fatalf("List(%s): got 0 rows, want at least 1", purposeFalse)
		}
		for _, r := range rows {
			if r.OneOfDomain {
				t.Errorf("List(%s): row %+v has OneOfDomain = true, want false", purposeFalse, r)
			}
		}
	})

	t.Run("no_policy_row", func(t *testing.T) {
		// No scope filter: purposeNone's only row is global.
		rows, err := integrationSvcs.TagTemplate.List(ctx, coreQ, tagQ, purposeNone, nil)
		if err != nil {
			t.Fatalf("List(%s): %v", purposeNone, err)
		}
		if len(rows) == 0 {
			t.Fatalf("List(%s): got 0 rows, want at least 1", purposeNone)
		}
		for _, r := range rows {
			if r.OneOfDomain {
				t.Errorf("List(%s): row %+v has OneOfDomain = true, want false (COALESCE default, no policy row)", purposeNone, r)
			}
		}
	})
}

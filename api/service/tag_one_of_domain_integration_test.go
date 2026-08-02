//go:build integration

package service

// tag_one_of_domain_integration_test.go proves the real Postgres
// tags_enforce_one_of_domain trigger (model/migrations/0205_tags_one_of_domain.sql)
// raises SQLSTATE 23505 end-to-end through TagService.Create against a live
// database, and that the existing Go-level classification
// (tag.go's errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation check)
// picks it up as ErrConflict with zero Go code changes to the classification
// logic itself (per plan/overview.md's Design decision 3/4). This
// complements tag_test.go's TestTagService_Create_OneOfDomainConflict*
// tests, which exercise the same Go-level classification against the mock's
// simulated conflict rather than the real trigger.
//
// This file reuses tag_grant_integration_test.go's shared TestMain (in the
// same package, same build tag) for the ephemeral composed-migrations
// directory, connectivity probing, and integrationPool/integrationSvcs/
// seedActor/mustEntityUUID setup -- see that file for those details. It does
// not define its own TestMain.
//
// Run with (from mod-tags/api):
//
//	go test -tags=integration -p 1 -v ./service/...

import (
	"context"
	"errors"
	"testing"

	"github.com/moduleforge/core-api/opctx"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// TestTagOneOfDomainIntegration seeds a tag_purpose_policies row with
// one_of_domain=true for a test purpose, inserts one tag for
// (owner, subject, purpose) through the real TagService.Create, then asserts
// a second Create call for the same (owner, subject, purpose) (different
// value) returns ErrConflict -- proving the real DB trigger (not the mock's
// simulation) raises SQLSTATE 23505 and that the existing Go-level
// classification picks it up end-to-end.
func TestTagOneOfDomainIntegration(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool
	coreQ := coredb.New(pool)
	tagQ := tagsdb.New(pool)

	const purpose = "one-of-domain-verify"

	if _, err := integrationSvcs.TagPurposePolicy.Upsert(ctx, tagQ, purpose, true); err != nil {
		t.Fatalf("seed tag_purpose_policies: %v", err)
	}

	owner, err := seedActor(ctx, pool, "ood-owner")
	if err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	subject, err := seedActor(ctx, pool, "ood-subject")
	if err != nil {
		t.Fatalf("seed subject: %v", err)
	}
	subjectUUID := mustEntityUUID(ctx, t, pool, subject)

	// First tag for (owner, subject, purpose) succeeds.
	if _, err := integrationSvcs.Tag.Create(opctx.WithActor(ctx, owner), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID, Purpose: purpose, Value: "first",
	}); err != nil {
		t.Fatalf("Create first tag: %v", err)
	}

	// Second tag for the same (owner, subject, purpose) -- different value --
	// must be rejected by the real trigger and classified as ErrConflict.
	_, err = integrationSvcs.Tag.Create(opctx.WithActor(ctx, owner), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID, Purpose: purpose, Value: "second",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict from real DB trigger, got %v", err)
	}
}

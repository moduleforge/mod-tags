package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	tagsdb "github.com/moduleforge/tags-model/db"
)

// TagPurposePolicy is the service-layer representation of a
// tag_purpose_policies row.
type TagPurposePolicy struct {
	Purpose     string
	OneOfDomain bool
}

// TagPurposePolicyServicer defines access to the tag_purpose_policies
// registry: an internal Get (used by callers that need to check/display a
// specific purpose's policy) and an internal administrative Upsert. Neither
// is exposed via any HTTP route in this plan — one_of_domain is a curated,
// admin-set value, not end-user or public-API-writable; values are
// seeded/managed out-of-band (see plan/overview.md's Design decision 4 and
// next-steps.md). Kept as its own type (not folded into TagTemplateServicer)
// because tag_purpose_policies is a distinct table/concept from
// tag_templates — see plan/overview.md's "Naming note".
type TagPurposePolicyServicer interface {
	// Get returns the policy for purpose, or a zero-value TagPurposePolicy
	// with OneOfDomain: false when no row exists — the same default the DB
	// trigger applies (see model/migrations/0205_tags_one_of_domain.sql).
	// Never an error for the "no row" case; only for genuine query failures.
	Get(ctx context.Context, tagQ tagsdb.Querier, purpose string) (TagPurposePolicy, error)

	// Upsert inserts or updates the one_of_domain flag for purpose.
	// Internal/administrative only — mirrors TagTemplateServicer.Upsert's
	// doc-commented convention exactly; no route calls it.
	Upsert(ctx context.Context, tagQ tagsdb.Querier, purpose string, oneOfDomain bool) (TagPurposePolicy, error)
}

// TagPurposePolicyService implements TagPurposePolicyServicer. Like
// TagTemplateService, it makes no Authorizer call: this table has no
// per-row authorization, is an internal/administrative capability with no
// HTTP-exposed route, and carries no state of its own, so the zero value is
// ready to use.
type TagPurposePolicyService struct{}

// Compile-time assertion.
var _ TagPurposePolicyServicer = (*TagPurposePolicyService)(nil)

// Get looks up the tag_purpose_policies row for purpose via the generated
// GetTagPurposePolicy query.
func (s *TagPurposePolicyService) Get(
	ctx context.Context,
	tagQ tagsdb.Querier,
	purpose string,
) (TagPurposePolicy, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return TagPurposePolicy{}, fmt.Errorf("%w: purpose is required", ErrInvalidInput)
	}

	row, err := tagQ.GetTagPurposePolicy(ctx, purpose)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TagPurposePolicy{Purpose: purpose, OneOfDomain: false}, nil
		}
		return TagPurposePolicy{}, fmt.Errorf("tagpurposepolicy.Get query: %w", err)
	}

	return hydrateTagPurposePolicy(row), nil
}

// Upsert inserts or updates the one_of_domain flag for purpose via the
// generated UpsertTagPurposePolicy query.
func (s *TagPurposePolicyService) Upsert(
	ctx context.Context,
	tagQ tagsdb.Querier,
	purpose string,
	oneOfDomain bool,
) (TagPurposePolicy, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return TagPurposePolicy{}, fmt.Errorf("%w: purpose is required", ErrInvalidInput)
	}

	row, err := tagQ.UpsertTagPurposePolicy(ctx, tagsdb.UpsertTagPurposePolicyParams{
		Purpose:     purpose,
		OneOfDomain: oneOfDomain,
	})
	if err != nil {
		return TagPurposePolicy{}, fmt.Errorf("tagpurposepolicy.Upsert query: %w", err)
	}

	return hydrateTagPurposePolicy(row), nil
}

// hydrateTagPurposePolicy converts a generated TagPurposePolicy row into the
// service-layer TagPurposePolicy type.
func hydrateTagPurposePolicy(r tagsdb.TagPurposePolicy) TagPurposePolicy {
	return TagPurposePolicy{
		Purpose:     r.Purpose,
		OneOfDomain: r.OneOfDomain,
	}
}

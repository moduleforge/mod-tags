package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// TagTemplate is the service-layer representation of a tag-templates catalog
// entry, exposing no internal ids. Scope is the owning app's public UUID;
// nil for a global (unscoped) template.
type TagTemplate struct {
	Purpose   string
	Value     string
	Label     string
	Color     *string
	SortOrder int32
	Scope     *uuid.UUID
}

// TagTemplateServicer defines access to the tag_templates catalog: an open
// read (List) plus a single administrative write (Upsert) for seeding
// app-scoped catalog rows. It is kept separate from TagServicer so the tags
// CRUD paths are unaffected by this addition.
type TagTemplateServicer interface {
	// List returns tag_templates rows matching purpose. When scope is nil,
	// only global templates are returned. When scope is a well-formed app
	// UUID, the result is global templates plus that app's scoped templates;
	// a well-formed but unknown scope UUID resolves as if scope were absent
	// (forgiving open-read) rather than as an error.
	List(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, purpose string, scope *uuid.UUID) ([]TagTemplate, error)

	// Upsert inserts or updates a single app-scoped tag_templates row, keyed
	// by (scope, purpose, value): a second call with the same scope/purpose/
	// value updates label/color/sort_order in place rather than creating a
	// duplicate row. Scope is required — this write path does not support
	// global (unscoped) templates — and must resolve to a known app entity;
	// an unknown or ill-formed scope is a genuine error here, unlike List's
	// forgiving-unknown-scope read behavior. It is an internal/administrative
	// capability meant for in-process callers (e.g. a consuming app's own
	// startup seeding hook), not an HTTP-exposed operation; no route calls
	// it.
	Upsert(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, in UpsertTagTemplateInput) (TagTemplate, error)
}

// UpsertTagTemplateInput describes one app-scoped tag_templates row to
// insert or update. Scope is required (see TagTemplateServicer.Upsert).
type UpsertTagTemplateInput struct {
	Scope     uuid.UUID
	Purpose   string
	Value     string
	Label     string
	Color     *string
	SortOrder int32
}

// TagTemplateService implements TagTemplateServicer. Its List method is an
// open read: unlike TagService, it makes no Authorizer call and applies no
// accessible_* row filtering — the catalog is not access-controlled. Upsert
// deliberately follows the same no-Authorizer convention: it is an
// internal/administrative write, called in-process by a consuming app's own
// startup hook rather than from an httpapi handler, so there is no HTTP-
// layer actor to authorize against. It carries no state of its own, so the
// zero value is ready to use.
type TagTemplateService struct{}

// Compile-time assertion.
var _ TagTemplateServicer = (*TagTemplateService)(nil)

// List resolves the optional scope UUID to an internal apps.id (via the core
// querier's entity lookup) and calls the generated ListTagTemplates query.
func (s *TagTemplateService) List(
	ctx context.Context,
	coreQ coredb.Querier,
	tagQ tagsdb.Querier,
	purpose string,
	scope *uuid.UUID,
) ([]TagTemplate, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		return nil, fmt.Errorf("%w: purpose is required", ErrInvalidInput)
	}

	// scopeParam's zero value (Valid: false) is NULL — used both when scope
	// is absent and when a well-formed scope UUID does not resolve.
	var scopeParam pgtype.Int8
	if scope != nil {
		entity, err := coreQ.GetEntityByUUID(ctx, *scope)
		switch {
		case err == nil:
			scopeParam = pgtype.Int8{Int64: entity.ID, Valid: true}
		case errors.Is(err, pgx.ErrNoRows):
			// Well-formed but unknown scope: forgiving open-read — fall
			// through with a NULL scope so the caller still gets globals.
		default:
			return nil, fmt.Errorf("tagtemplate.List resolve scope: %w", err)
		}
	}

	rows, err := tagQ.ListTagTemplates(ctx, tagsdb.ListTagTemplatesParams{
		Purpose: purpose,
		Scope:   scopeParam,
	})
	if err != nil {
		return nil, fmt.Errorf("tagtemplate.List query: %w", err)
	}

	result := make([]TagTemplate, 0, len(rows))
	for _, row := range rows {
		result = append(result, hydrateTagTemplate(row))
	}
	return result, nil
}

// hydrateTagTemplate converts a generated ListTagTemplatesRow into the
// service-layer TagTemplate type.
func hydrateTagTemplate(r tagsdb.ListTagTemplatesRow) TagTemplate {
	t := TagTemplate{
		Purpose:   r.Purpose,
		Value:     r.Value,
		Label:     r.Label,
		SortOrder: r.SortOrder,
	}
	if r.Color.Valid {
		c := r.Color.String
		t.Color = &c
	}
	if r.ScopeUuid.Valid {
		u := uuid.UUID(r.ScopeUuid.Bytes)
		t.Scope = &u
	}
	return t
}

// Upsert resolves in.Scope to an internal apps.id (via the core querier's
// entity lookup) and calls the generated UpsertTagTemplate query, returning
// the hydrated result. Unlike List's scope resolution, an unknown scope is
// a genuine error: a write path has no reasonable "fall through" behavior
// the way a forgiving read does.
func (s *TagTemplateService) Upsert(
	ctx context.Context,
	coreQ coredb.Querier,
	tagQ tagsdb.Querier,
	in UpsertTagTemplateInput,
) (TagTemplate, error) {
	purpose := strings.TrimSpace(in.Purpose)
	value := strings.TrimSpace(in.Value)
	label := strings.TrimSpace(in.Label)
	switch {
	case purpose == "":
		return TagTemplate{}, fmt.Errorf("%w: purpose is required", ErrInvalidInput)
	case value == "":
		return TagTemplate{}, fmt.Errorf("%w: value is required", ErrInvalidInput)
	case label == "":
		return TagTemplate{}, fmt.Errorf("%w: label is required", ErrInvalidInput)
	case in.Scope == uuid.Nil:
		return TagTemplate{}, fmt.Errorf("%w: scope is required", ErrInvalidInput)
	}

	entity, err := coreQ.GetEntityByUUID(ctx, in.Scope)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TagTemplate{}, fmt.Errorf("%w: scope %s does not resolve to a known app", ErrInvalidInput, in.Scope)
		}
		return TagTemplate{}, fmt.Errorf("tagtemplate.Upsert resolve scope: %w", err)
	}

	var colorParam pgtype.Text
	if in.Color != nil {
		colorParam = pgtype.Text{String: *in.Color, Valid: true}
	}

	row, err := tagQ.UpsertTagTemplate(ctx, tagsdb.UpsertTagTemplateParams{
		Scope:     pgtype.Int8{Int64: entity.ID, Valid: true},
		Purpose:   purpose,
		Value:     value,
		Label:     label,
		Color:     colorParam,
		SortOrder: in.SortOrder,
	})
	if err != nil {
		return TagTemplate{}, fmt.Errorf("tagtemplate.Upsert query: %w", err)
	}

	return hydrateUpsertedTagTemplate(row, in.Scope), nil
}

// hydrateUpsertedTagTemplate converts a generated UpsertTagTemplateRow into
// the service-layer TagTemplate type. scope is threaded through from the
// caller's input rather than re-derived from the row (UpsertTagTemplateRow
// carries the internal scope id, not the app's public UUID).
func hydrateUpsertedTagTemplate(r tagsdb.UpsertTagTemplateRow, scope uuid.UUID) TagTemplate {
	t := TagTemplate{
		Purpose:   r.Purpose,
		Value:     r.Value,
		Label:     r.Label,
		SortOrder: r.SortOrder,
		Scope:     &scope,
	}
	if r.Color.Valid {
		c := r.Color.String
		t.Color = &c
	}
	return t
}

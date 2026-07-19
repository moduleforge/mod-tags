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

// TagTemplateServicer defines read-only access to the tag_templates catalog.
// It is kept separate from TagServicer so the tags CRUD paths are unaffected
// by this addition.
type TagTemplateServicer interface {
	// List returns tag_templates rows matching purpose. When scope is nil,
	// only global templates are returned. When scope is a well-formed app
	// UUID, the result is global templates plus that app's scoped templates;
	// a well-formed but unknown scope UUID resolves as if scope were absent
	// (forgiving open-read) rather than as an error.
	List(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, purpose string, scope *uuid.UUID) ([]TagTemplate, error)
}

// TagTemplateService implements TagTemplateServicer as an open read: unlike
// TagService, it makes no Authorizer call and applies no accessible_* row
// filtering — the catalog is not access-controlled. It carries no state of
// its own, so the zero value is ready to use.
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

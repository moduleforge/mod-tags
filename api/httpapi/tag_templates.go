package httpapi

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/tags-api/service"
)

// tagTemplateResponse is the JSON shape returned for a single tag_templates
// catalog entry. No internal id is emitted; scope is the owning app's public
// UUID, or null for a global template.
type tagTemplateResponse struct {
	Purpose     string  `json:"purpose"`
	Value       string  `json:"value"`
	Label       string  `json:"label"`
	Color       *string `json:"color"`
	SortOrder   int32   `json:"sortOrder"`
	Scope       *string `json:"scope"`
	OneOfDomain bool    `json:"oneOfDomain"`
}

func toTagTemplateResponse(t service.TagTemplate) tagTemplateResponse {
	resp := tagTemplateResponse{
		Purpose:     t.Purpose,
		Value:       t.Value,
		Label:       t.Label,
		Color:       t.Color,
		SortOrder:   t.SortOrder,
		OneOfDomain: t.OneOfDomain,
	}
	if t.Scope != nil {
		s := t.Scope.String()
		resp.Scope = &s
	}
	return resp
}

// handleListTagTemplates handles GET /tag-templates. Any authenticated actor
// may read the catalog: unlike the tags endpoints, there is no per-row
// authorization — the tag_templates catalog is not access-controlled beyond
// requiring an authenticated actor.
func (h *handlers) handleListTagTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	q := r.URL.Query()

	purpose := q.Get("purpose")
	if purpose == "" {
		apiresp.WriteError(w, r, fmt.Errorf("%w: purpose is required", apiresp.ErrInvalidInput))
		return
	}

	var scope *uuid.UUID
	if scopeStr := q.Get("scope"); scopeStr != "" {
		parsed, err := uuid.Parse(scopeStr)
		if err != nil {
			apiresp.WriteError(w, r, fmt.Errorf("%w: scope must be a valid UUID", apiresp.ErrInvalidInput))
			return
		}
		scope = &parsed
	}

	templates, err := h.d.Services.TagTemplate.List(
		r.Context(),
		h.d.CoreQuerier,
		h.d.Services.Querier(),
		purpose,
		scope,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	resp := make([]tagTemplateResponse, 0, len(templates))
	for _, t := range templates {
		resp = append(resp, toTagTemplateResponse(t))
	}
	apiresp.WriteJSON(w, http.StatusOK, resp)
}

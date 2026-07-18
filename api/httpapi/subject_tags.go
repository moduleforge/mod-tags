package httpapi

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/opctx"
)

// handleSubjectTags handles GET /entities/{uuid}/tags.
// Returns all tags whose subject is the given entity UUID, filtered by authz.
func (h *handlers) handleSubjectTags(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	subjectUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid uuid", apiresp.ErrInvalidInput))
		return
	}

	q := r.URL.Query()

	var purposeFilter *string
	if pStr := q.Get("purpose"); pStr != "" {
		s := pStr
		purposeFilter = &s
	}

	pag := parsePagination(q)

	tags, err := h.d.Services.Tag.ListBySubject(
		r.Context(),
		h.d.CoreQuerier,
		h.d.Services.Querier(),
		subjectUUID,
		purposeFilter,
		pag,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	resp := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		resp = append(resp, toTagResponse(t))
	}
	apiresp.WriteJSON(w, http.StatusOK, map[string]any{"tags": resp})
}

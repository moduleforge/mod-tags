package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/tags-api/service"
)

// tagResponse is the JSON shape returned for a single tag.
type tagResponse struct {
	UUID        string  `json:"uuid"`
	OwnerUUID   string  `json:"ownerUuid"`
	SubjectUUID string  `json:"subjectUuid"`
	Purpose     string  `json:"purpose"`
	Value       string  `json:"value"`
	Color       *string `json:"color"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	OneOfDomain bool    `json:"oneOfDomain"`
}

func toTagResponse(t service.Tag) tagResponse {
	return tagResponse{
		UUID:        t.EntityUUID.String(),
		OwnerUUID:   t.OwnerUUID.String(),
		SubjectUUID: t.SubjectUUID.String(),
		Purpose:     t.Purpose,
		Value:       t.Value,
		Color:       t.Color,
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		OneOfDomain: t.OneOfDomain,
	}
}

// createTagRequest is the body for POST /tags.
type createTagRequest struct {
	Subject string  `json:"subject"` // subject entity UUID
	Purpose string  `json:"purpose"`
	Value   string  `json:"value"`
	Color   *string `json:"color"`
}

// handleCreateTag handles POST /tags.
func (h *handlers) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createTagRequest
	if err := dec.Decode(&req); err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid JSON body", apiresp.ErrInvalidInput))
		return
	}

	subjectUUID, err := uuid.Parse(req.Subject)
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: subject must be a valid UUID", apiresp.ErrInvalidInput))
		return
	}

	in := service.CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           req.Purpose,
		Value:             req.Value,
		Color:             req.Color,
	}

	tag, err := h.d.Services.Tag.Create(
		r.Context(),
		h.d.CoreQuerier,
		in,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	apiresp.WriteJSON(w, http.StatusCreated, toTagResponse(tag))
}

// handleSearchTags handles GET /tags.
func (h *handlers) handleSearchTags(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	q := r.URL.Query()

	filter := service.SearchTagsFilter{}

	if ownerStr := q.Get("owner"); ownerStr != "" {
		parsed, err := uuid.Parse(ownerStr)
		if err != nil {
			apiresp.WriteError(w, r, fmt.Errorf("%w: owner must be a valid UUID", apiresp.ErrInvalidInput))
			return
		}
		filter.OwnerEntityUUID = &parsed
	}
	if subjectStr := q.Get("subject"); subjectStr != "" {
		parsed, err := uuid.Parse(subjectStr)
		if err != nil {
			apiresp.WriteError(w, r, fmt.Errorf("%w: subject must be a valid UUID", apiresp.ErrInvalidInput))
			return
		}
		filter.SubjectEntityUUID = &parsed
	}
	if purposeStr := q.Get("purpose"); purposeStr != "" {
		s := purposeStr
		filter.Purpose = &s
	}
	if valueStr := q.Get("value"); valueStr != "" {
		s := valueStr
		filter.Value = &s
	}

	pag := parsePagination(q)

	tags, err := h.d.Services.Tag.Search(
		r.Context(),
		h.d.CoreQuerier,
		h.d.Services.Querier(),
		filter,
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
	apiresp.WriteJSON(w, http.StatusOK, resp)
}

// handleGetTag handles GET /tags/{uuid}.
func (h *handlers) handleGetTag(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid uuid", apiresp.ErrInvalidInput))
		return
	}

	tag, err := h.d.Services.Tag.GetByUUID(
		r.Context(),
		h.d.CoreQuerier,
		h.d.Services.Querier(),
		entityUUID,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	apiresp.WriteJSON(w, http.StatusOK, toTagResponse(tag))
}

// updateTagRequest is the strict body for PUT /tags/{uuid}.
// Only the "color" key is accepted; any other key causes a 400 via
// DisallowUnknownFields. Color is a json.RawMessage so we can distinguish
// absent (→ 400) from null (→ clear) from a string value (→ set).
type updateTagRequest struct {
	Color json.RawMessage `json:"color"`
}

// handlePutTag handles PUT /tags/{uuid}.
func (h *handlers) handlePutTag(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid uuid", apiresp.ErrInvalidInput))
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req updateTagRequest
	if err := dec.Decode(&req); err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: only 'color' is accepted in the request body", apiresp.ErrInvalidInput))
		return
	}

	// Absent color field → 400.
	if len(req.Color) == 0 {
		apiresp.WriteError(w, r, fmt.Errorf("%w: color field is required in body", apiresp.ErrInvalidInput))
		return
	}

	// Determine whether color is null (clear) or a string (set).
	var colorValue *string
	if !bytes.Equal(bytes.TrimSpace(req.Color), []byte("null")) {
		var s string
		if err := json.Unmarshal(req.Color, &s); err != nil {
			apiresp.WriteError(w, r, fmt.Errorf("%w: color must be a string or null", apiresp.ErrInvalidInput))
			return
		}
		colorValue = &s
	}

	in := service.UpdateTagInput{Color: colorValue}

	tag, err := h.d.Services.Tag.UpdateByUUID(
		r.Context(),
		h.d.CoreQuerier,
		entityUUID,
		in,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	apiresp.WriteJSON(w, http.StatusOK, toTagResponse(tag))
}

// patchTagRequest is the strict body for PATCH /tags/{uuid}.
// Only the "value" key is accepted; any other key causes a 400 via
// DisallowUnknownFields. Value is required (NOT NULL) so absent or null → 400.
type patchTagRequest struct {
	Value *string `json:"value"`
}

// handlePatchTag handles PATCH /tags/{uuid}.
func (h *handlers) handlePatchTag(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid uuid", apiresp.ErrInvalidInput))
		return
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req patchTagRequest
	if err := dec.Decode(&req); err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: only 'value' is accepted in the request body", apiresp.ErrInvalidInput))
		return
	}

	// Absent or null value → 400; value is NOT NULL and required.
	if req.Value == nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: value is required in body", apiresp.ErrInvalidInput))
		return
	}

	in := service.UpdateTagValueInput{Value: *req.Value}

	tag, err := h.d.Services.Tag.UpdateTagValue(
		r.Context(),
		h.d.CoreQuerier,
		entityUUID,
		in,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	apiresp.WriteJSON(w, http.StatusOK, toTagResponse(tag))
}

// handleDeleteTag handles DELETE /tags/{uuid}.
func (h *handlers) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if _, ok := opctx.ActorEntityID(r.Context()); !ok {
		apiresp.WriteError(w, r, apiresp.ErrUnauthenticated)
		return
	}

	entityUUID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		apiresp.WriteError(w, r, fmt.Errorf("%w: invalid uuid", apiresp.ErrInvalidInput))
		return
	}

	err = h.d.Services.Tag.DeleteByUUID(
		r.Context(),
		h.d.CoreQuerier,
		entityUUID,
	)
	if err != nil {
		apiresp.WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parsePagination reads ?limit=&offset= from a query-string map and returns a
// Pagination value. Non-numeric or absent values are treated as zero (Pagination
// will apply defaults via normalize()).
func parsePagination(q interface{ Get(string) string }) service.Pagination {
	var p service.Pagination
	if s := q.Get("limit"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 32); err == nil {
			p.Limit = int32(v)
		}
	}
	if s := q.Get("offset"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 32); err == nil {
			p.Offset = int32(v)
		}
	}
	return p
}

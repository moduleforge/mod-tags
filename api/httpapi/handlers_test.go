package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/tags-api/service"
)

// withActor injects an actor entity ID into the request's context.
func withActor(r *http.Request, entityID int64) *http.Request {
	return r.WithContext(opctx.WithActor(r.Context(), entityID))
}

// --- POST /tags ---

func TestHandleCreateTag_201_HappyPath(t *testing.T) {
	tagUUID := uuid.New()
	ownerUUID := uuid.New()
	subjectUUID := uuid.New()
	svc := &fakeTagService{tag: service.Tag{
		EntityUUID:  tagUUID,
		OwnerUUID:   ownerUUID,
		SubjectUUID: subjectUUID,
		Purpose:     "label",
		Value:       "urgent",
	}}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{
		"subject": subjectUUID.String(),
		"purpose": "label",
		"value":   "urgent",
	})
	req := httptest.NewRequest(http.MethodPost, "/tags", bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestHandleCreateTag_401_Unauthenticated(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{"subject": uuid.New().String(), "purpose": "x", "value": "y"})
	req := httptest.NewRequest(http.MethodPost, "/tags", bytes.NewBuffer(body))
	// no actor injected — unauthenticated
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateTag_400_MissingSubject(t *testing.T) {
	svc := &fakeTagService{err: fmt.Errorf("%w: purpose is required", service.ErrInvalidInput)}
	router := NewRouter(buildTestDeps(svc))

	// Subject is not a valid UUID.
	body, _ := json.Marshal(map[string]any{"subject": "not-a-uuid", "purpose": "x", "value": "y"})
	req := httptest.NewRequest(http.MethodPost, "/tags", bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTag_400_BadColorFromService(t *testing.T) {
	svc := &fakeTagService{err: fmt.Errorf("%w: color must match #RRGGBBAA", service.ErrInvalidInput)}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{
		"subject": uuid.New().String(),
		"purpose": "label",
		"value":   "v",
		"color":   "#ZZZZZZZZ",
	})
	req := httptest.NewRequest(http.MethodPost, "/tags", bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTag_400_UnknownField(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{
		"subject": uuid.New().String(),
		"purpose": "label",
		"value":   "urgent",
		"owner":   uuid.New().String(), // unknown field; should be rejected
	})
	req := httptest.NewRequest(http.MethodPost, "/tags", bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// --- GET /tags ---

func TestHandleSearchTags_400_NoFilter(t *testing.T) {
	svc := &fakeTagService{err: fmt.Errorf("%w: at least one of owner or subject is required", service.ErrInvalidInput)}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/tags", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleSearchTags_200_EmptyResult(t *testing.T) {
	svc := &fakeTagService{tags: []service.Tag{}}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/tags?owner="+uuid.New().String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleSearchTags_400_InvalidOwnerUUID(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	req := httptest.NewRequest(http.MethodGet, "/tags?owner=not-a-uuid", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- GET /tags/{uuid} ---

func TestHandleGetTag_200_Authorized(t *testing.T) {
	tagUUID := uuid.New()
	svc := &fakeTagService{tag: service.Tag{
		EntityUUID:  tagUUID,
		OwnerUUID:   uuid.New(),
		SubjectUUID: uuid.New(),
		Purpose:     "p",
		Value:       "v",
	}}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/tags/"+tagUUID.String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetTag_404_Unauthorized(t *testing.T) {
	svc := &fakeTagService{err: service.ErrNotFound}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/tags/"+uuid.New().String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- PUT /tags/{uuid} ---

func TestHandlePutTag_200_HappyPath(t *testing.T) {
	tagUUID := uuid.New()
	color := "#FF0000FF"
	svc := &fakeTagService{tag: service.Tag{
		EntityUUID:  tagUUID,
		OwnerUUID:   uuid.New(),
		SubjectUUID: uuid.New(),
		Purpose:     "p",
		Value:       "v",
		Color:       &color,
	}}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{"color": color})
	req := httptest.NewRequest(http.MethodPut, "/tags/"+tagUUID.String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

// TestHandlePutTag_400_ImmutabilityBypass is the required test from the spec:
// PUT body with {"color":"#FF0000FF","purpose":"new"} must return 400.
func TestHandlePutTag_400_ImmutabilityBypass(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{"color": "#FF0000FF", "purpose": "new"})
	req := httptest.NewRequest(http.MethodPut, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePutTag_400_UnknownKey(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{"mystery_field": "value"})
	req := httptest.NewRequest(http.MethodPut, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlePutTag_400_AbsentColor(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	// Empty object: color key absent — must be 400.
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPut, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePutTag_200_NullColorClears(t *testing.T) {
	tagUUID := uuid.New()
	// Service returns a tag with no color — confirming clear was applied.
	svc := &fakeTagService{tag: service.Tag{
		EntityUUID:  tagUUID,
		OwnerUUID:   uuid.New(),
		SubjectUUID: uuid.New(),
		Purpose:     "p",
		Value:       "v",
		Color:       nil,
	}}
	router := NewRouter(buildTestDeps(svc))

	// Explicit null: {"color": null} — must be 200 (clear color).
	req := httptest.NewRequest(http.MethodPut, "/tags/"+tagUUID.String(),
		bytes.NewBufferString(`{"color":null}`))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandlePutTag_403_SubjectTries(t *testing.T) {
	svc := &fakeTagService{err: service.ErrForbidden}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{"color": "#FF0000FF"})
	req := httptest.NewRequest(http.MethodPut, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// --- PATCH /tags/{uuid} ---

func TestHandlePatchTag_200_HappyPath(t *testing.T) {
	tagUUID := uuid.New()
	svc := &fakeTagService{tag: service.Tag{
		EntityUUID:  tagUUID,
		OwnerUUID:   uuid.New(),
		SubjectUUID: uuid.New(),
		Purpose:     "status",
		Value:       "active",
	}}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{"value": "active"})
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+tagUUID.String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandlePatchTag_400_UnknownField(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{"value": "active", "purpose": "new"})
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePatchTag_400_MissingValue(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	// Empty object: value key absent — must be 400.
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePatchTag_400_NullValue(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	// Explicit null: {"value": null} — value is NOT NULL, must be 400.
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+uuid.New().String(),
		bytes.NewBufferString(`{"value":null}`))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePatchTag_404_NotFound(t *testing.T) {
	svc := &fakeTagService{err: service.ErrNotFound}
	router := NewRouter(buildTestDeps(svc))

	body, _ := json.Marshal(map[string]any{"value": "active"})
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	req = withActor(req, 1)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePatchTag_401_Unauthenticated(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	body, _ := json.Marshal(map[string]any{"value": "active"})
	req := httptest.NewRequest(http.MethodPatch, "/tags/"+uuid.New().String(), bytes.NewBuffer(body))
	// no actor injected — unauthenticated
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- DELETE /tags/{uuid} ---

func TestHandleDeleteTag_204_Success(t *testing.T) {
	svc := &fakeTagService{err: nil}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodDelete, "/tags/"+uuid.New().String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHandleDeleteTag_403_SubjectForbidden(t *testing.T) {
	svc := &fakeTagService{err: service.ErrForbidden}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodDelete, "/tags/"+uuid.New().String(), nil)
	req = withActor(req, 2)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleDeleteTag_404_Stranger(t *testing.T) {
	svc := &fakeTagService{err: service.ErrNotFound}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodDelete, "/tags/"+uuid.New().String(), nil)
	req = withActor(req, 3)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- GET /entities/{uuid}/tags ---

func TestHandleSubjectTags_200_WithTags(t *testing.T) {
	svc := &fakeTagService{tags: []service.Tag{
		{EntityUUID: uuid.New(), OwnerUUID: uuid.New(), SubjectUUID: uuid.New(), Purpose: "p", Value: "v"},
	}}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/tags", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["tags"]; !ok {
		t.Error("response missing 'tags' key")
	}
}

func TestHandleSubjectTags_404_UnknownSubject(t *testing.T) {
	svc := &fakeTagService{err: service.ErrNotFound}
	router := NewRouter(buildTestDeps(svc))

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/tags", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleSubjectTags_401_Unauthenticated(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))

	req := httptest.NewRequest(http.MethodGet, "/entities/"+uuid.New().String()+"/tags", nil)
	// no actor injected — unauthenticated
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

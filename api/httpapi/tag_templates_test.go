package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/moduleforge/tags-api/service"
)

// --- GET /tag-templates ---

func TestHandleListTagTemplates_400_MissingPurpose(t *testing.T) {
	router := NewRouter(buildTestDepsForTemplates(nil))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleListTagTemplates_200_PurposeOnly_Globals(t *testing.T) {
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{
		{Purpose: "status", Value: "active", Label: "Active", SortOrder: 0, Scope: nil},
		{Purpose: "status", Value: "blocked", Label: "Blocked", SortOrder: 1, Scope: nil},
	}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body.String())
	}
	if len(body) != 2 {
		t.Fatalf("len(body): got %d, want 2", len(body))
	}
	for _, row := range body {
		if row["scope"] != nil {
			t.Errorf("row scope: got %v, want nil (JSON null)", row["scope"])
		}
		if _, ok := row["id"]; ok {
			t.Error("response leaks an internal 'id' field")
		}
	}
}

func TestHandleListTagTemplates_200_PurposeAndScope_GlobalsPlusScoped(t *testing.T) {
	appUUID := uuid.New()
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{
		{Purpose: "status", Value: "active", Label: "Active", SortOrder: 0, Scope: nil},
		{Purpose: "status", Value: "custom", Label: "Custom", SortOrder: 1, Scope: &appUUID},
	}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status&scope="+appUUID.String(), nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body.String())
	}
	if len(body) != 2 {
		t.Fatalf("len(body): got %d, want 2", len(body))
	}

	var sawGlobal, sawScoped bool
	for _, row := range body {
		switch row["value"] {
		case "active":
			if row["scope"] != nil {
				t.Errorf("global row scope: got %v, want nil", row["scope"])
			}
			sawGlobal = true
		case "custom":
			if row["scope"] != appUUID.String() {
				t.Errorf("scoped row scope: got %v, want %q", row["scope"], appUUID.String())
			}
			sawScoped = true
		}
	}
	if !sawGlobal || !sawScoped {
		t.Errorf("response missing expected rows: sawGlobal=%v sawScoped=%v; body=%v", sawGlobal, sawScoped, body)
	}
}

// TestHandleListTagTemplates_200_OneOfDomainTrue confirms a template row
// with OneOfDomain: true round-trips through the wire response as
// oneOfDomain: true, and that the response body is still a bare JSON array
// (starts with '[', not an object envelope).
func TestHandleListTagTemplates_200_OneOfDomainTrue(t *testing.T) {
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{
		{Purpose: "priority", Value: "urgent", Label: "Urgent", SortOrder: 0, Scope: nil, OneOfDomain: true},
	}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=priority", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if !strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "[") {
		t.Fatalf("response body is not a bare JSON array: %s", rec.Body.String())
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body.String())
	}
	if len(body) != 1 {
		t.Fatalf("len(body): got %d, want 1", len(body))
	}

	val, ok := body[0]["oneOfDomain"]
	if !ok {
		t.Fatalf("row missing oneOfDomain key: %v", body[0])
	}
	if val != true {
		t.Errorf("row oneOfDomain: got %v, want true", val)
	}
}

// TestHandleListTagTemplates_200_OneOfDomainFalse confirms a template row
// with OneOfDomain: false still emits the oneOfDomain key with value false
// rather than omitting it — a wrongly-added `omitempty` on the response
// struct field would silently drop a false value, which is exactly the bug
// this assertion catches via the two-value map lookup.
func TestHandleListTagTemplates_200_OneOfDomainFalse(t *testing.T) {
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{
		{Purpose: "status", Value: "active", Label: "Active", SortOrder: 0, Scope: nil, OneOfDomain: false},
	}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v; body: %s", err, rec.Body.String())
	}
	if len(body) != 1 {
		t.Fatalf("len(body): got %d, want 1", len(body))
	}

	val, ok := body[0]["oneOfDomain"]
	if !ok {
		t.Fatalf("row missing oneOfDomain key (omitempty would drop a false value): %v", body[0])
	}
	if val != false {
		t.Errorf("row oneOfDomain: got %v, want false", val)
	}
}

func TestHandleListTagTemplates_400_MalformedScope(t *testing.T) {
	router := NewRouter(buildTestDepsForTemplates(nil))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status&scope=not-a-uuid", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleListTagTemplates_200_EmptyResult(t *testing.T) {
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=nonexistent", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("len(body): got %d, want 0", len(body))
	}
}

func TestHandleListTagTemplates_500_ServiceError(t *testing.T) {
	svc := &fakeTagTemplateService{err: fmt.Errorf("boom")}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status", nil)
	req = withActor(req, 1)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

// TestHandleListTagTemplates_401_Unauthenticated confirms that, like the
// tags endpoints, a request with no actor in context is rejected before
// any per-row authorization would even be considered — the catalog has no
// per-row authz, but it still requires an authenticated actor.
func TestHandleListTagTemplates_401_Unauthenticated(t *testing.T) {
	svc := &fakeTagTemplateService{templates: []service.TagTemplate{}}
	router := NewRouter(buildTestDepsForTemplates(svc))

	req := httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=status", nil)
	// no actor injected — unauthenticated
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

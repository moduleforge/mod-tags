package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// tgAuthnCase is one anonymous request against one mod-tags route.
type tgAuthnCase struct {
	name   string
	method string
	path   string
	body   string
}

// tgAuthnCases returns, for all 8 routes, a well-formed request plus the
// malformed variants that today's handlers answer with 400 (bad path UUID,
// bad body, bad query). With the mount in place every one is a 401.
func tgAuthnCases() []tgAuthnCase {
	id := uuid.New().String()
	valid := `{"subject":"` + id + `","purpose":"p","value":"v","color":"#aabbccdd"}`
	return []tgAuthnCase{
		// Well-formed.
		{"POST /tags", http.MethodPost, "/tags", valid},
		{"GET /tags", http.MethodGet, "/tags?owner=" + id, ""},
		{"GET /tags/{uuid}", http.MethodGet, "/tags/" + id, ""},
		{"PUT /tags/{uuid}", http.MethodPut, "/tags/" + id, `{"color":"#aabbccdd"}`},
		{"PATCH /tags/{uuid}", http.MethodPatch, "/tags/" + id, `{"value":"v"}`},
		{"DELETE /tags/{uuid}", http.MethodDelete, "/tags/" + id, ""},
		{"GET /entities/{uuid}/tags", http.MethodGet, "/entities/" + id + "/tags", ""},
		{"GET /tag-templates", http.MethodGet, "/tag-templates?purpose=p", ""},
		// Malformed: would be 400 without the gate.
		{"POST /tags malformed body", http.MethodPost, "/tags", `{not json`},
		{"POST /tags bad subject", http.MethodPost, "/tags", `{"subject":"bad","purpose":"p","value":"v"}`},
		{"GET /tags bad owner", http.MethodGet, "/tags?owner=bad", ""},
		{"GET /tags/{uuid} bad uuid", http.MethodGet, "/tags/not-a-uuid", ""},
		{"PUT /tags/{uuid} bad uuid", http.MethodPut, "/tags/not-a-uuid", `{"color":"#aabbccdd"}`},
		{"PUT /tags/{uuid} malformed body", http.MethodPut, "/tags/" + id, `{not json`},
		{"PATCH /tags/{uuid} bad uuid", http.MethodPatch, "/tags/not-a-uuid", `{"value":"v"}`},
		{"PATCH /tags/{uuid} malformed body", http.MethodPatch, "/tags/" + id, `{not json`},
		{"DELETE /tags/{uuid} bad uuid", http.MethodDelete, "/tags/not-a-uuid", ""},
		{"GET /entities/{uuid}/tags bad uuid", http.MethodGet, "/entities/not-a-uuid/tags", ""},
		{"GET /tag-templates missing purpose", http.MethodGet, "/tag-templates", ""},
		{"GET /tag-templates bad scope", http.MethodGet, "/tag-templates?purpose=p&scope=bad", ""},
	}
}

func tgAuthnServe(t *testing.T, h http.Handler, c tgAuthnCase) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
	if c.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func tgAuthnRun(t *testing.T, h http.Handler) {
	t.Helper()
	for _, c := range tgAuthnCases() {
		t.Run(c.name, func(t *testing.T) {
			rec := tgAuthnServe(t, h, c)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("anonymous %s %s: status %d, want 401; body: %s", c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTgAuthn_NewRouter_AnonymousGets401(t *testing.T) {
	tgAuthnRun(t, NewRouter(buildTestDeps(nil)))
}

func TestTgAuthn_RegisterRoutes_BareRouter_AnonymousGets401(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, buildTestDeps(nil))
	tgAuthnRun(t, r)
}

func TestTgAuthn_RegisterRoutes_DoesNotGateSiblingRoutes(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, buildTestDeps(nil))
	r.Get("/dummy", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	rec := tgAuthnServe(t, r, tgAuthnCase{method: http.MethodGet, path: "/dummy"})
	if rec.Code != http.StatusNoContent {
		t.Errorf("anonymous GET /dummy: status %d, want 204 (route must not be gated)", rec.Code)
	}
	rec = tgAuthnServe(t, r, tgAuthnCase{method: http.MethodGet, path: "/tag-templates?purpose=p"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous GET /tag-templates: status %d, want 401", rec.Code)
	}
}

// An authenticated caller passes the gate (positive control: the 401s above
// are the mount's doing, not a blanket rejection).
func TestTgAuthn_Authenticated_PassesGate(t *testing.T) {
	h := NewRouter(buildTestDeps(nil))
	req := withActor(httptest.NewRequest(http.MethodGet, "/tag-templates?purpose=p", nil), 1)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated GET /tag-templates: status %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

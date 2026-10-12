package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/opctx"
)

// tgInlCase is one well-formed request against one mod-tags route. A sudo-only
// identity (no regular actor, a sudo actor only) must pass the RequireAuth
// gate mounted by NewRouter and reach the (fake) service, which answers 2xx.
type tgInlCase struct {
	name   string
	method string
	path   string
	body   string
}

func tgInlCases() []tgInlCase {
	id := uuid.New().String()
	valid := `{"subject":"` + id + `","purpose":"p","value":"v","color":"#aabbccdd"}`
	return []tgInlCase{
		{"POST /tags", http.MethodPost, "/tags", valid},
		{"GET /tags", http.MethodGet, "/tags?owner=" + id, ""},
		{"GET /tags/{uuid}", http.MethodGet, "/tags/" + id, ""},
		{"PUT /tags/{uuid}", http.MethodPut, "/tags/" + id, `{"color":"#aabbccdd"}`},
		{"PATCH /tags/{uuid}", http.MethodPatch, "/tags/" + id, `{"value":"v"}`},
		{"DELETE /tags/{uuid}", http.MethodDelete, "/tags/" + id, ""},
		{"GET /entities/{uuid}/tags", http.MethodGet, "/entities/" + id + "/tags", ""},
		{"GET /tag-templates", http.MethodGet, "/tag-templates?purpose=p", ""},
	}
}

// TestTgInl_NewRouter_SudoOnlyIsNotUnauthenticated: a sudo-only identity is
// authenticated (sudo-aware RequireAuth), so none of the 8 routes answers 401.
// The fake services return success, so each well-formed request is 2xx,
// proving the request reached the service layer.
func TestTgInl_NewRouter_SudoOnlyIsNotUnauthenticated(t *testing.T) {
	router := NewRouter(buildTestDeps(nil))
	for _, c := range tgInlCases() {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
			if c.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req = req.WithContext(opctx.WithSudoActor(req.Context(), 99))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("sudo-only %s %s: status 401, want authenticated; body: %s", c.method, c.path, rec.Body.String())
			}
			if rec.Code/100 != 2 {
				t.Errorf("sudo-only %s %s: status %d, want 2xx (reaches fake service); body: %s", c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/apiresp"
)

// tgShapeServe sends an authenticated request through NewRouter and decodes
// the error envelope (if any).
func tgShapeServe(t *testing.T, method, path, body string) (*httptest.ResponseRecorder, apiresp.Envelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	NewRouter(buildTestDeps(nil)).ServeHTTP(rec, withActor(req, 1))
	var env apiresp.Envelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec, env
}

func tgShapeWantDetail(t *testing.T, rec *httptest.ResponseRecorder, env apiresp.Envelope, code, field string) {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	if env.Error.Code != "invalid_input" {
		t.Errorf("top-level code %q, want invalid_input", env.Error.Code)
	}
	if len(env.Error.Details) != 1 {
		t.Fatalf("details %+v, want exactly one entry; body: %s", env.Error.Details, rec.Body.String())
	}
	d := env.Error.Details[0]
	if d.Code != code {
		t.Errorf("detail code %q, want %q", d.Code, code)
	}
	if field != "" && d.Field != field {
		t.Errorf("detail field %q, want %q", d.Field, field)
	}
}

func TestTgShape_BodyRoutes_OverCapBody(t *testing.T) {
	big := `{"subject":"` + uuid.New().String() + `","purpose":"` + strings.Repeat("a", 2<<20) + `","value":"v"}`
	cases := []struct{ name, method, path, body string }{
		{"POST", http.MethodPost, "/tags", big},
		{"PUT", http.MethodPut, "/tags/" + uuid.New().String(), `{"color":"` + strings.Repeat("a", 2<<20) + `"}`},
		{"PATCH", http.MethodPatch, "/tags/" + uuid.New().String(), `{"value":"` + strings.Repeat("a", 2<<20) + `"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, env := tgShapeServe(t, c.method, c.path, c.body)
			tgShapeWantDetail(t, rec, env, "tags.body_too_large", "")
		})
	}
}

func TestTgShape_BodyRoutes_TrailingDataAndUnknownFields(t *testing.T) {
	id := uuid.New().String()
	cases := []struct{ name, method, path, body string }{
		{"POST trailing", http.MethodPost, "/tags", `{"subject":"` + id + `","purpose":"p","value":"v"} {}`},
		{"POST unknown", http.MethodPost, "/tags", `{"subject":"` + id + `","purpose":"p","value":"v","bogus":1}`},
		{"PUT trailing", http.MethodPut, "/tags/" + id, `{"color":null} x`},
		{"PUT unknown", http.MethodPut, "/tags/" + id, `{"color":null,"bogus":1}`},
		{"PATCH trailing", http.MethodPatch, "/tags/" + id, `{"value":"v"} {}`},
		{"PATCH unknown", http.MethodPatch, "/tags/" + id, `{"value":"v","bogus":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, env := tgShapeServe(t, c.method, c.path, c.body)
			tgShapeWantDetail(t, rec, env, "tags.invalid_json_body", "")
		})
	}
}

func TestTgShape_MalformedPathUUID_FieldUUID(t *testing.T) {
	cases := []struct{ name, method, path, body string }{
		{"GET", http.MethodGet, "/tags/nope", ""},
		{"PUT", http.MethodPut, "/tags/nope", `{"color":null}`},
		{"PATCH", http.MethodPatch, "/tags/nope", `{"value":"v"}`},
		{"DELETE", http.MethodDelete, "/tags/nope", ""},
		{"GET subject tags", http.MethodGet, "/entities/nope/tags", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, env := tgShapeServe(t, c.method, c.path, c.body)
			tgShapeWantDetail(t, rec, env, "invalid_uuid", "uuid")
		})
	}
}

func TestTgShape_MalformedQueryAndBodyUUIDs(t *testing.T) {
	id := uuid.New().String()
	cases := []struct{ name, method, path, body, field string }{
		{"owner", http.MethodGet, "/tags?owner=nope", "", "owner"},
		{"subject query", http.MethodGet, "/tags?subject=nope", "", "subject"},
		{"scope", http.MethodGet, "/tag-templates?purpose=p&scope=nope", "", "scope"},
		{"subject body", http.MethodPost, "/tags", `{"subject":"nope","purpose":"p","value":"` + id + `"}`, "subject"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, env := tgShapeServe(t, c.method, c.path, c.body)
			tgShapeWantDetail(t, rec, env, "invalid_uuid", c.field)
		})
	}
}

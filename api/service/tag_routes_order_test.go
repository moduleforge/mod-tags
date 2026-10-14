package service_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moduleforge/core-api/apiresp/ordertest"
)

// tgOrdUUIDRoutes builds the Routes for the five routes keyed by a path UUID.
// Every miss on these routes is masked to 403 (D4: entityResolver.Resolve
// default policy), hence AuthorizedMissingStatus 403. Each Route gets a fresh
// fixture because DELETE mutates the seeded tag.
func tgOrdUUIDRoutes() []ordertest.Route {
	mk := func(name, method, existing, missing string, good, bad string) ordertest.Route {
		env := orderRouter()
		r := ordertest.Route{
			Name:                    name,
			Handler:                 env.router,
			Method:                  method,
			Authorized:              env.owner,
			Unauthorized:            env.stranger,
			AuthorizedMissingStatus: http.StatusForbidden,
		}
		r.ExistingPath = env.pathFor(existing)
		r.MissingPath = env.pathFor(missing)
		if good != "" {
			r.GoodBody = []byte(good)
		}
		if bad != "" {
			r.BadBody = []byte(bad)
		}
		return r
	}
	return []ordertest.Route{
		mk("GET /tags/{uuid}", http.MethodGet, "tag", "missingTag", "", ""),
		// BadBody is a decode-layer rejection (malformed JSON).
		mk("PUT /tags/{uuid}", http.MethodPut, "tag", "missingTag", `{"color":"#11223344"}`, `{"color":`),
		mk("PATCH /tags/{uuid}", http.MethodPatch, "tag", "missingTag", `{"value":"renamed"}`, `{"value":`),
		mk("DELETE /tags/{uuid}", http.MethodDelete, "tag", "missingTag", "", ""),
		mk("GET /entities/{uuid}/tags", http.MethodGet, "subjectTags", "missingSubjectTags", "", ""),
	}
}

// pathFor maps a symbolic path name to the fixture's path.
func (e *orderEnv) pathFor(name string) string {
	switch name {
	case "tag":
		return e.tagPath
	case "missingTag":
		return e.missingTag
	case "subjectTags":
		return e.subjectTagsPath
	case "missingSubjectTags":
		return e.missingSubjectTagsPath
	}
	panic("unknown path name " + name)
}

func TestOrder_UUIDRoutes(t *testing.T) {
	ordertest.Run(t, tgOrdUUIDRoutes()...)
}

// TestOrder_ExtraStatelessRules adds one Route per distinct stateless rule
// beyond the decode layer: handler rules (PUT without color, PATCH without
// value, bad subject UUID) and service rules (bad color, blank value/purpose,
// NUL). Each must be a 400 for the stranger on both the existing and the
// missing path (stateless before authorization/resolve), 401 for anonymous.
func TestOrder_ExtraStatelessRules(t *testing.T) {
	var routes []ordertest.Route
	add := func(name, method string, existingName, missingName, good, bad string) {
		env := orderRouter()
		routes = append(routes, ordertest.Route{
			Name:                    name,
			Handler:                 env.router,
			Method:                  method,
			ExistingPath:            env.pathFor(existingName),
			MissingPath:             env.pathFor(missingName),
			GoodBody:                []byte(good),
			BadBody:                 []byte(bad),
			Authorized:              env.owner,
			Unauthorized:            env.stranger,
			AuthorizedMissingStatus: http.StatusForbidden,
		})
	}
	add("PUT /tags/{uuid} without color (handler)", http.MethodPut, "tag", "missingTag",
		`{"color":"#11223344"}`, `{}`)
	add("PUT /tags/{uuid} color wrong type (handler)", http.MethodPut, "tag", "missingTag",
		`{"color":"#11223344"}`, `{"color":5}`)
	add("PUT /tags/{uuid} bad color (service)", http.MethodPut, "tag", "missingTag",
		`{"color":"#11223344"}`, `{"color":"red"}`)
	add("PATCH /tags/{uuid} without value (handler)", http.MethodPatch, "tag", "missingTag",
		`{"value":"renamed"}`, `{}`)
	add("PATCH /tags/{uuid} blank value (service)", http.MethodPatch, "tag", "missingTag",
		`{"value":"renamed"}`, `{"value":"   "}`)
	add("PUT /tags/{uuid} unknown field (decode)", http.MethodPut, "tag", "missingTag",
		`{"color":"#11223344"}`, `{"color":null,"bogus":1}`)
	ordertest.Run(t, routes...)
}

// tgOrdCreateBody renders a POST /tags body for the fixture.
func tgOrdCreateBody(env *orderEnv, subject, purpose, value string, color *string) string {
	m := map[string]any{"subject": subject, "purpose": purpose, "value": value}
	if color != nil {
		m["color"] = *color
	}
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestOrder_PostTags covers POST /tags, which has no path id and so cannot be
// modelled as an existing/missing pair: 401, then 400 (decode layer, handler
// subject-UUID rule, service rules: blank purpose, bad color), then 403, and
// the sudo-only 403 (D3). The one-of-domain 409 is a late rejection that only
// an authorized caller sees (403 for the stranger, 401 for anonymous).
func TestOrder_PostTags(t *testing.T) {
	env := orderRouter()
	good := tgOrdCreateBody(env, env.subjectUUID.String(), "label", "x", nil)
	badColor := "red"
	bad := map[string]string{
		"malformed json":   `{"subject":`,
		"unknown field":    `{"subject":"` + env.subjectUUID.String() + `","purpose":"p","value":"v","bogus":1}`,
		"empty body":       ``,
		"bad subject uuid": tgOrdCreateBody(env, "not-a-uuid", "label", "x", nil),
		"blank purpose":    tgOrdCreateBody(env, env.subjectUUID.String(), "  ", "x", nil),
		"blank value":      tgOrdCreateBody(env, env.subjectUUID.String(), "label", "  ", nil),
		"bad color":        tgOrdCreateBody(env, env.subjectUUID.String(), "label", "x", &badColor),
		"purpose with NUL": tgOrdCreateBody(env, env.subjectUUID.String(), "la\x00bel", "x", nil),
	}
	for name, body := range bad {
		t.Run("bad body/"+name, func(t *testing.T) {
			orderExpect(t, env.router, ordertest.Anonymous, http.MethodPost, "/tags", body, http.StatusUnauthorized)
			// Stateless rejection precedes authorization: the stranger sees 400.
			orderExpect(t, env.router, env.stranger, http.MethodPost, "/tags", body, http.StatusBadRequest)
			orderExpect(t, env.router, env.owner, http.MethodPost, "/tags", body, http.StatusBadRequest)
		})
	}
	t.Run("anonymous good body", func(t *testing.T) {
		orderExpect(t, env.router, ordertest.Anonymous, http.MethodPost, "/tags", good, http.StatusUnauthorized)
	})
	t.Run("stranger good body is 403", func(t *testing.T) {
		orderExpect(t, env.router, env.stranger, http.MethodPost, "/tags", good, http.StatusForbidden)
	})
	t.Run("sudo-only is authenticated but has no owner: 403 not 401", func(t *testing.T) {
		orderExpect(t, env.router, ordertest.SudoOnly(1<<40), http.MethodPost, "/tags", good, http.StatusForbidden)
	})
	t.Run("authorized good body is 201", func(t *testing.T) {
		orderExpect(t, env.router, env.owner, http.MethodPost, "/tags", good, http.StatusCreated)
	})
}

// TestOrder_PostTags_Late covers the one-of-domain 409: the fakes model the
// tags_enforce_one_of_domain trigger (mockTagQuerier.CreateTag raises SQLSTATE
// 23505 for a second tag by the same owner on the same subject and purpose
// under a one-of-domain policy), so the full late-rejection ordering is
// asserted through ordertest. Both paths are the collection path because
// POST /tags has no path id.
func TestOrder_PostTags_Late(t *testing.T) {
	env := orderRouter()
	conflict := tgOrdCreateBody(env, env.subjectUUID.String(), orderOneOfDomainPurpose, "second", nil)
	good := tgOrdCreateBody(env, env.subjectUUID.String(), "label", "x", nil)
	ordertest.Run(t, ordertest.Route{
		Name:                    "POST /tags",
		Handler:                 env.router,
		Method:                  http.MethodPost,
		ExistingPath:            "/tags",
		MissingPath:             "/tags",
		GoodBody:                []byte(good),
		BadBody:                 []byte(`{"subject":`),
		Authorized:              env.owner,
		Unauthorized:            env.stranger,
		AuthorizedMissingStatus: http.StatusCreated,
		Late: []ordertest.Late{
			{Name: "one_of_domain", Path: "/tags", Body: []byte(conflict), WantStatus: http.StatusConflict},
		},
	})
}

// TestOrder_PostTags_SubjectNotAuthorized_p3Gt PINS THE CURRENT BEHAVIOR of
// finding p3Gt: POST /tags authorizes "create" on the ACTOR's own entity id
// only and never on the subject. After authorization it resolves the subject
// through the masked resolver, so for an authenticated actor who may create
// tags the response distinguishes an existing subject (201) from a missing one
// (masked 403, the same body a denied caller gets), and the actor needs no
// permission on the subject. If the manager changes p3Gt (for example by
// authorizing the subject), THIS is the test to update.
func TestOrder_PostTags_SubjectNotAuthorized_p3Gt(t *testing.T) {
	env := orderRouter()
	t.Run("existing subject: 201", func(t *testing.T) {
		body := tgOrdCreateBody(env, env.subjectUUID.String(), "label", "p3gt-existing", nil)
		orderExpect(t, env.router, env.owner, http.MethodPost, "/tags", body, http.StatusCreated)
	})
	t.Run("existing subject with one-of-domain conflict: 409", func(t *testing.T) {
		body := tgOrdCreateBody(env, env.subjectUUID.String(), orderOneOfDomainPurpose, "p3gt-conflict", nil)
		orderExpect(t, env.router, env.owner, http.MethodPost, "/tags", body, http.StatusConflict)
	})
	t.Run("missing subject: masked 403", func(t *testing.T) {
		body := tgOrdCreateBody(env, env.missingSubjectUUID.String(), "label", "p3gt-missing", nil)
		orderExpect(t, env.router, env.owner, http.MethodPost, "/tags", body, http.StatusForbidden)
	})
	t.Run("denied caller: 403 whether or not the subject exists", func(t *testing.T) {
		for _, u := range []string{env.subjectUUID.String(), env.missingSubjectUUID.String()} {
			body := tgOrdCreateBody(env, u, orderOneOfDomainPurpose, "p3gt-denied", nil)
			orderExpect(t, env.router, env.stranger, http.MethodPost, "/tags", body, http.StatusForbidden)
		}
	})
}

// TestOrder_GetTags covers GET /tags (collection route, no path id): 401, then
// 400 (malformed filter UUID; "neither owner nor subject"), then 403, plus the
// sudo-only 403 (D3).
func TestOrder_GetTags(t *testing.T) {
	env := orderRouter()
	ownerFilter := "/tags?owner=" + env.subjectUUID.String()
	cases := []struct {
		name string
		id   ordertest.Identity
		path string
		want int
	}{
		{"anonymous", ordertest.Anonymous, ownerFilter, http.StatusUnauthorized},
		{"anonymous neither owner nor subject", ordertest.Anonymous, "/tags", http.StatusUnauthorized},
		{"anonymous malformed owner", ordertest.Anonymous, "/tags?owner=nope", http.StatusUnauthorized},
		{"stranger neither owner nor subject", env.stranger, "/tags", http.StatusBadRequest},
		{"owner neither owner nor subject", env.owner, "/tags", http.StatusBadRequest},
		{"stranger malformed owner", env.stranger, "/tags?owner=nope", http.StatusBadRequest},
		{"stranger malformed subject", env.stranger, "/tags?subject=nope", http.StatusBadRequest},
		{"stranger NUL purpose", env.stranger, ownerFilter + "&purpose=%00", http.StatusBadRequest},
		{"stranger valid filter", env.stranger, ownerFilter, http.StatusForbidden},
		{"sudo-only valid filter", ordertest.SudoOnly(1 << 40), ownerFilter, http.StatusForbidden},
		{"owner valid filter", env.owner, ownerFilter, http.StatusOK},
		{"owner filter by unknown subject", env.owner, "/tags?subject=" + env.missingSubjectUUID.String(), http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orderExpect(t, env.router, tc.id, http.MethodGet, tc.path, "", tc.want)
		})
	}
}

// TestOrder_MalformedPathUUID covers the stateless-shape slot for the five
// path-UUID routes, which ordertest cannot model: authn (401) precedes shape
// (400), and shape precedes authorization, so even the stranger sees 400.
func TestOrder_MalformedPathUUID(t *testing.T) {
	env := orderRouter()
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/tags/not-a-uuid", ""},
		{http.MethodPut, "/tags/not-a-uuid", `{"color":null}`},
		{http.MethodPatch, "/tags/not-a-uuid", `{"value":"v"}`},
		{http.MethodDelete, "/tags/not-a-uuid", ""},
		{http.MethodGet, "/entities/not-a-uuid/tags", ""},
	}
	for _, rt := range routes {
		for _, tc := range []struct {
			name string
			id   ordertest.Identity
			want int
		}{
			{"anonymous", ordertest.Anonymous, http.StatusUnauthorized},
			{"stranger", env.stranger, http.StatusBadRequest},
			{"owner", env.owner, http.StatusBadRequest},
		} {
			t.Run(fmt.Sprintf("%s %s/%s", rt.method, rt.path, tc.name), func(t *testing.T) {
				orderExpect(t, env.router, tc.id, rt.method, rt.path, rt.body, tc.want)
			})
		}
	}
}

// TestOrder_SubjectTags_NULPurpose covers the stateless purpose rule on
// GET /entities/{uuid}/tags: it precedes the subject resolve and the
// authorization, so a stranger sees 400 for an existing and a missing subject
// alike, and anonymous still sees 401.
func TestOrder_SubjectTags_NULPurpose(t *testing.T) {
	env := orderRouter()
	for _, p := range []string{env.subjectTagsPath, env.missingSubjectTagsPath} {
		path := p + "?purpose=%00"
		orderExpect(t, env.router, ordertest.Anonymous, http.MethodGet, path, "", http.StatusUnauthorized)
		orderExpect(t, env.router, env.stranger, http.MethodGet, path, "", http.StatusBadRequest)
		orderExpect(t, env.router, env.owner, http.MethodGet, path, "", http.StatusBadRequest)
	}
}

// TestOrder_SubjectTags_SudoOnly pins the D3 service guard on
// GET /entities/{uuid}/tags: a sudo-only identity is authenticated (not 401)
// but has no regular actor to scope rows by, so it is 403.
func TestOrder_SubjectTags_SudoOnly(t *testing.T) {
	env := orderRouter()
	orderExpect(t, env.router, ordertest.SudoOnly(1<<40), http.MethodGet, env.subjectTagsPath, "", http.StatusForbidden)
}

// TestOrder_ListTagTemplates covers GET /tag-templates, which is
// authentication-only by design (open catalog): anonymous is 401 even with a
// missing purpose, an authenticated caller gets 400 for a missing purpose or
// a malformed scope, a sudo-only identity is served (200), and Authorize is
// never called, so there is no 403 stage.
func TestOrder_ListTagTemplates(t *testing.T) {
	env := orderRouter()
	sudoOnly := ordertest.SudoOnly(1 << 40)
	cases := []struct {
		name string
		id   ordertest.Identity
		path string
		want int
	}{
		{"anonymous missing purpose", ordertest.Anonymous, "/tag-templates", http.StatusUnauthorized},
		{"anonymous malformed scope", ordertest.Anonymous, "/tag-templates?purpose=label&scope=nope", http.StatusUnauthorized},
		{"anonymous valid", ordertest.Anonymous, "/tag-templates?purpose=label", http.StatusUnauthorized},
		{"authenticated missing purpose", env.stranger, "/tag-templates", http.StatusBadRequest},
		{"authenticated blank purpose", env.stranger, "/tag-templates?purpose=%20", http.StatusBadRequest},
		{"authenticated bad scope", env.stranger, "/tag-templates?purpose=label&scope=nope", http.StatusBadRequest},
		{"authenticated valid", env.stranger, "/tag-templates?purpose=label", http.StatusOK},
		{"owner valid with unknown scope", env.owner, "/tag-templates?purpose=label&scope=" + env.missingUUID.String(), http.StatusOK},
		{"sudo-only missing purpose", sudoOnly, "/tag-templates", http.StatusBadRequest},
		{"sudo-only valid", sudoOnly, "/tag-templates?purpose=label", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orderExpect(t, env.router, tc.id, http.MethodGet, tc.path, "", tc.want)
		})
	}
	if n := env.az.calls.Load(); n != 0 {
		t.Fatalf("Authorize was called %d times on GET /tag-templates, want 0", n)
	}
}

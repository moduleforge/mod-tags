package httpapi

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/moduleforge/core-api/authz"
	coredb "github.com/moduleforge/core-model/db"
	"github.com/moduleforge/tags-api/service"
)

// Deps carries the external dependencies that httpapi handlers need.
type Deps struct {
	// CoreQuerier is the base core-model Querier for entity/type resolution.
	CoreQuerier coredb.Querier
	// Services holds the tag CRUD implementations.
	Services *service.Services
	// Logger is the structured logger for handler-level error messages.
	Logger *slog.Logger
}

type handlers struct {
	d Deps
}

// NewRouter wires all tag routes on a fresh chi.Router and returns it. It
// delegates to RegisterRoutes, so authentication (authz.RequireAuth) is
// enforced identically through both entry points.
// Mount it under any prefix, e.g. r.Mount("/v1", tags.NewRouter(deps)).
func NewRouter(d Deps) chi.Router {
	r := chi.NewRouter()
	RegisterRoutes(r, d)
	return r
}

// RegisterRoutes registers all tag routes on the provided router. Unlike
// NewRouter it does not allocate a new mux, so the routes can be composed
// onto a caller-supplied chi.Router — for example nested under a shared /v1
// prefix alongside other modules' routes and gated by scope-based auth
// middleware. This is the entry point mfgen's register:-style route wiring
// calls.
//
// RegisterRoutes enforces authentication with authz.RequireAuth, so an
// unauthenticated request is rejected with 401 before any handler decoding or
// validation runs. The middleware is mounted on an inner chi group, never on
// the supplied router itself, so routes the caller registers beside these
// stay ungated.
func RegisterRoutes(r chi.Router, d Deps) {
	h := &handlers{d: d}

	r.Group(func(r chi.Router) {
		r.Use(authz.RequireAuth)

		r.Post("/tags", h.handleCreateTag)
		r.Get("/tags", h.handleSearchTags)
		r.Get("/tags/{uuid}", h.handleGetTag)
		r.Put("/tags/{uuid}", h.handlePutTag)
		r.Patch("/tags/{uuid}", h.handlePatchTag)
		r.Delete("/tags/{uuid}", h.handleDeleteTag)
		r.Get("/entities/{uuid}/tags", h.handleSubjectTags)
		// Authenticated but, by design, not authorized per caller (open catalog).
		r.Get("/tag-templates", h.handleListTagTemplates)
	})
}

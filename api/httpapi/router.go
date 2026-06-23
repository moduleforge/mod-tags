package httpapi

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

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

// NewRouter wires all tag routes and returns a mountable chi.Router.
// Mount it under any prefix, e.g. r.Mount("/v1", tags.NewRouter(deps)).
func NewRouter(d Deps) chi.Router {
	r := chi.NewRouter()
	h := &handlers{d: d}

	r.Post("/tags", h.handleCreateTag)
	r.Get("/tags", h.handleSearchTags)
	r.Get("/tags/{uuid}", h.handleGetTag)
	r.Put("/tags/{uuid}", h.handlePutTag)
	r.Patch("/tags/{uuid}", h.handlePatchTag)
	r.Delete("/tags/{uuid}", h.handleDeleteTag)
	r.Get("/entities/{uuid}/tags", h.handleSubjectTags)

	return r
}

// Package httpapi provides the chi subrouter for tag endpoints.
// Consumers call NewRouter(deps) and mount the returned router under their
// preferred path prefix (typically /v1), or call RegisterRoutes to compose the
// routes onto an existing router. Both entry points enforce authentication with
// authz.RequireAuth, so unauthenticated requests get 401 before any other
// rejection.
package httpapi

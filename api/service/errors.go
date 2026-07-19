package service

import "github.com/moduleforge/core-api/apiresp"

// ErrNotFound is returned when a requested resource does not exist or is not
// visible to the caller (to avoid leaking existence). It is an alias of the
// canonical apiresp.ErrNotFound sentinel.
var ErrNotFound = apiresp.ErrNotFound

// ErrForbidden is returned when the caller lacks permission for the operation
// but is known to have visibility of the resource (e.g., subject trying PUT).
// It is an alias of the canonical apiresp.ErrForbidden sentinel.
var ErrForbidden = apiresp.ErrForbidden

// ErrUnauthenticated is returned when no actor could be established on the
// context (the "no actor on context" defensive checks in tag.go's
// Create/Search/ListBySubject). It is an alias of the canonical
// apiresp.ErrUnauthenticated sentinel; apiresp.WriteError maps this to 401,
// distinct from ErrForbidden's 403 — see
// docs/mf-standards/architecture/api-response-design.md's "401 vs 403"
// section.
var ErrUnauthenticated = apiresp.ErrUnauthenticated

// ErrInvalidInput is returned when the caller supplies invalid or missing
// input. It is an alias of the canonical apiresp.ErrInvalidInput sentinel.
var ErrInvalidInput = apiresp.ErrInvalidInput

// ErrConflict is returned when a uniqueness constraint would be violated. It
// is an alias of the canonical apiresp.ErrConflict sentinel.
var ErrConflict = apiresp.ErrConflict

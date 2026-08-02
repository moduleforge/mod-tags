// Package service exposes tx-aware tag CRUD for consumer apps.
// Consumer modules wire in their own auth and audit implementations
// via the authz.Authorizer and observer.MutationObserver interfaces.
package service

import (
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/txhelper"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// Services is the aggregate of all tag service implementations. Consumers
// construct this once at startup and pass it into httpapi.NewRouter.
type Services struct {
	Tag TagServicer

	// TagTemplate is the read-only tag_templates catalog service. It has no
	// dependencies of its own (open read: no authz, no per-row filtering)
	// and is kept separate from Tag so the tags CRUD paths are unaffected.
	TagTemplate TagTemplateServicer

	// TagPurposePolicy is the tag_purpose_policies registry service. It has
	// no dependencies of its own (no authz, no per-row filtering) and is
	// kept separate so the tags CRUD paths are unaffected.
	TagPurposePolicy TagPurposePolicyServicer

	// q is the base Querier backed by the pool, exposed so handlers can
	// derive tx-scoped queriers via tagsdb.New(tx).
	q tagsdb.Querier
}

// New constructs a Services aggregate.
//
// coreQ is typically coredb.New(pool) and is the base querier for pre-tx
// entity resolution reads.
//
// tagQ is typically tagsdb.New(pool) and is the base querier for pre-tx
// tag reads.
//
// db is the connection pool (or any txhelper.DB) used to open transactions
// for mutating operations.
//
// az gates every operation; a non-nil error from az.Authorize aborts the
// operation immediately.
//
// opRes provides the satisfied-by closure for list queries. Pass the
// OperationRegistry loaded at startup from the authz_operations table.
// List queries call opRes.SatisfiedBy("read") to compute the op_ids slice
// passed to the access function: anyone with a read, sread, update, delete,
// swrite, or manage grant can see the row.
//
// obs receives in-tx and post-commit notifications for every mutation;
// pass observer.NewObserverGroup() for a no-op group.
//
// resolver is the startup-time type-ID cache; used to resolve "tag" to its
// internal type ID for Authorize targets.
//
// entityResolver translates an entity UUID to its internal entity ID for
// UUID-keyed reads (GetByUUID), applying the configured per-resource not-
// found policy (default: 403 to mask existence).
func New(coreQ coredb.Querier, tagQ tagsdb.Querier, db txhelper.DB, az authz.Authorizer, opRes authz.OpResolver, obs *observer.ObserverGroup, resolver *types.Resolver, entityResolver *entity.Resolver) *Services {
	newCoreQ := func(tx pgx.Tx) coredb.Querier { return coredb.New(tx) }
	newTagQ := func(tx pgx.Tx) tagsdb.Querier { return tagsdb.New(tx) }
	return &Services{
		Tag: &TagService{
			db:             db,
			az:             az,
			opRes:          opRes,
			obs:            obs,
			resolver:       resolver,
			entityResolver: entityResolver,
			newCoreQuerier: newCoreQ,
			newTagQuerier:  newTagQ,
		},
		TagTemplate:      &TagTemplateService{},
		TagPurposePolicy: &TagPurposePolicyService{},
		q:                tagQ,
	}
}

// Querier returns the base Querier so handlers can derive tx-scoped variants.
func (s *Services) Querier() tagsdb.Querier {
	return s.q
}

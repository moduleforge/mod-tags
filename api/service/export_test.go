package service

// Test-only exports for the external order-test package (service_test). They
// expose the in-package fakes (defined in mock_test.go) and a constructor that
// builds *Services like New but with the tx-querier factories pointed at the
// fakes, so the real httpapi router can be driven over the real services
// without a database. Every exported symbol starts with Order.

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/txhelper"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// Aliases for the fakes the order tests need.
type (
	OrderFakeDB       = fakeDB
	OrderCoreQuerier  = mockCoreQuerier
	OrderTagQuerier   = mockTagQuerier
	OrderOpResolver   = stubOpResolver
	OrderRecordingObs = recordingObserver
)

// OrderNewFakeDB returns a txhelper.DB whose transactions commit successfully.
func OrderNewFakeDB() *OrderFakeDB { return newFakeDB() }

// OrderNewCoreQuerier returns an empty mock core querier with the type
// registry seeded.
func OrderNewCoreQuerier() *OrderCoreQuerier { return newMockCoreQuerier() }

// OrderNewTagQuerier returns an empty mock tags querier.
func OrderNewTagQuerier() *OrderTagQuerier { return newMockTagQuerier() }

// OrderNewOpResolver returns the stub op resolver used by list queries.
func OrderNewOpResolver() *OrderOpResolver { return newStubOpResolver() }

// OrderNewServices builds a *Services exactly as New does, then points the
// transaction-scoped querier factories at the supplied fakes (New's defaults
// would wrap the fake transaction in the real sqlc queriers).
func OrderNewServices(
	coreQ *OrderCoreQuerier,
	tagQ *OrderTagQuerier,
	db txhelper.DB,
	az authz.Authorizer,
	opRes authz.OpResolver,
	obs *observer.ObserverGroup,
) *Services {
	resolver, err := types.New(context.Background(), coreQ)
	if err != nil {
		panic("OrderNewServices: " + err.Error())
	}
	svcs := New(coreQ, tagQ, db, az, opRes, obs, resolver, entity.NewResolver())
	ts := svcs.Tag.(*TagService)
	ts.newCoreQuerier = func(pgx.Tx) coredb.Querier { return coreQ }
	ts.newTagQuerier = func(pgx.Tx) tagsdb.Querier { return tagQ }
	return svcs
}

// OrderSeedEntity inserts an entity of the given fundamental type slug and
// returns its public UUID and internal id. Ids are allocated sequentially from 1.
func OrderSeedEntity(q *OrderCoreQuerier, typeSlug string) (uuid.UUID, int64) {
	return q.seedEntity(typeSlug)
}

// OrderSeedTag seeds a tag entity plus its tags row and returns the tag's
// public UUID and internal entity id.
func OrderSeedTag(coreQ *OrderCoreQuerier, tagQ *OrderTagQuerier, ownerID, subjectID int64, purpose, value string) (uuid.UUID, int64) {
	_, entityID := coreQ.seedEntity("tag")
	tagUUID, _ := tagQ.seedTag(entityID, ownerID, subjectID, purpose, value, nil)
	// Make the tag's public UUID resolvable through the core entity lookup.
	coreQ.entities[tagUUID] = coreQ.entitiesByID[entityID]
	return tagUUID, entityID
}

// OrderSeedPurposePolicy sets the one-of-domain flag for a purpose.
func OrderSeedPurposePolicy(tagQ *OrderTagQuerier, purpose string, oneOfDomain bool) {
	tagQ.seedPurposePolicy(purpose, oneOfDomain)
}

// OrderSeedGlobalTemplate seeds a global (unscoped) tag template.
func OrderSeedGlobalTemplate(tagQ *OrderTagQuerier, purpose, value, label string) {
	tagQ.seedTemplate(purpose, value, label, nil, 1, 0, uuid.Nil)
}

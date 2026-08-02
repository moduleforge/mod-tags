package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// actorCtx returns a context with the given entity ID set as the actor.
func actorCtx(entityID int64) context.Context {
	return opctx.WithActor(context.Background(), entityID)
}

// mustResolver builds a types.Resolver from a mockCoreQuerier. Panics if the
// resolver cannot be constructed (indicates a bug in the mock setup).
func mustResolver(coreQ *mockCoreQuerier) *types.Resolver {
	r, err := types.New(context.Background(), coreQ)
	if err != nil {
		panic("mustResolver: " + err.Error())
	}
	return r
}

// buildService constructs a TagService with allow-all authz and a no-op observer,
// wired to the provided mocks. Returns a recording observer for assertion.
func buildService(coreQ *mockCoreQuerier, tagQ *mockTagQuerier) (*TagService, *recordingObserver) {
	rec := &recordingObserver{}
	obs := observer.NewObserverGroup(rec)
	svc := &TagService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		opRes:          newStubOpResolver(),
		obs:            obs,
		resolver:       mustResolver(coreQ),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	return svc, rec
}

// buildServiceWithTag sets up a mock environment with one seeded tag and returns
// the queriers, the entity UUIDs, and the service.
func buildServiceWithTag(t *testing.T) (svc *TagService, coreQ *mockCoreQuerier, tagQ *mockTagQuerier, tagUUID uuid.UUID, ownerID, subjectID, entityID int64) {
	t.Helper()
	coreQ = newMockCoreQuerier()
	tagQ = newMockTagQuerier()

	svc, _ = buildService(coreQ, tagQ)

	// Seed owner, subject entities in coreQ.
	_, ownerID = coreQ.seedEntity("natural_person")
	_, subjectID = coreQ.seedEntity("natural_person")

	// Seed tag entity.
	_, entityID = coreQ.seedEntity("tag")

	// Link entity UUID → tag ID mapping in tagQ.
	tagUUID, _ = tagQ.seedTag(entityID, ownerID, subjectID, "label", "urgent", nil)

	// Ensure coreQ knows about the tagUUID → entityID mapping.
	row := coreQ.entitiesByID[entityID]
	coreQ.entities[tagUUID] = row

	return
}

// --- Create validation tests ---

func TestTagService_Create_MissingPurpose(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(),
		Purpose:           "",
		Value:             "x",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Create_MissingValue(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(),
		Purpose:           "label",
		Value:             "",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Create_InvalidColor(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)
	badColor := "#ZZZZZZZZ"

	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(),
		Purpose:           "label",
		Value:             "v",
		Color:             &badColor,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Create_SubjectNotFound(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	// Seed owner entity.
	_, ownerID := coreQ.seedEntity("natural_person")

	// Unknown subject UUID. Existence-masking: entityResolver.Resolve
	// returns ErrForbidden (403), not ErrNotFound (404), on a genuine miss —
	// tags has not opted the "subject_entity" slug into AllowNotFound.
	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(), // not seeded
		Purpose:           "label",
		Value:             "v",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (subject UUID-not-found masked as 403), got %v", err)
	}
}

// TestTagService_Create_SetsEntityOwner asserts that Create writes ownership
// onto the centralized entities.owner_id at INSERT time (via
// CreateEntityWithOwner) in addition to the local tags.owner_id, both set
// from the same actor value. Setting ownership at INSERT time (rather than a
// follow-up UPDATE) is required because entities.owner_id is immutable after
// insert (entities_owner_immutable fires on any UPDATE OF owner_id,
// including the first NULL -> value write).
func TestTagService_Create_SetsEntityOwner(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")

	got, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "label",
		Value:             "urgent",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	entityRow, ok := coreQ.entities[got.EntityUUID]
	if !ok {
		t.Fatalf("entity not found in coreQ for uuid %s", got.EntityUUID)
	}

	if len(coreQ.createEntityWithOwnerCalls) != 1 {
		t.Fatalf("want 1 CreateEntityWithOwner call, got %d", len(coreQ.createEntityWithOwnerCalls))
	}
	call := coreQ.createEntityWithOwnerCalls[0]
	if !call.OwnerID.Valid || call.OwnerID.Int64 != ownerID {
		t.Errorf("CreateEntityWithOwner OwnerID = %+v, want valid Int64=%d", call.OwnerID, ownerID)
	}
	tagTypeID := mustResolver(coreQ).IDForSlugMust("tag")
	if call.FundamentalTypeID != tagTypeID {
		t.Errorf("CreateEntityWithOwner FundamentalTypeID = %d, want %d", call.FundamentalTypeID, tagTypeID)
	}

	// The local tags.owner_id write must be set from the same actor value.
	tagRow, ok := tagQ.tags[entityRow.ID]
	if !ok {
		t.Fatalf("tag row not found in tagQ for entity id %d", entityRow.ID)
	}
	if tagRow.OwnerID != ownerID {
		t.Errorf("local tags.owner_id = %d, want %d", tagRow.OwnerID, ownerID)
	}
}

// --- Authorize-denied tests ---

func TestTagService_Create_AuthorizeDenied(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		resolver:       mustResolver(coreQ),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(),
		Purpose:           "label",
		Value:             "v",
	})
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
	// No DB calls should have been made: tagQ should be empty.
	if len(tagQ.tags) != 0 {
		t.Error("expected no tags created when authz denied")
	}
}

func TestTagService_Update_AuthorizeDenied(t *testing.T) {
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	color := "#FF0000FF"

	_, err := svc.UpdateByUUID(actorCtx(999), coreQ, tagUUID, UpdateTagInput{Color: &color})
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
}

func TestTagService_Delete_AuthorizeDenied(t *testing.T) {
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	err := svc.DeleteByUUID(actorCtx(999), coreQ, tagUUID)
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
}

// --- In-tx observer error causes rollback ---

func TestTagService_Update_InTxObserverError_Propagates(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, _ := buildServiceWithTag(t)
	obsErr := errors.New("observer failure")
	rec := &recordingObserver{observeErr: obsErr}
	obs := observer.NewObserverGroup(rec) // default policy = PolicyPropagate
	svc := &TagService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            obs,
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	color := "#FF0000FF"

	_, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: &color})
	if !errors.Is(err, obsErr) {
		t.Errorf("want observer error propagated, got %v", err)
	}
	if len(rec.observeCalls) != 1 {
		t.Errorf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
}

// --- Observer records correct op/resource ---

func TestTagService_Update_ObserverCalled(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, _ := buildServiceWithTag(t)
	rec := &recordingObserver{}
	obs := observer.NewObserverGroup(rec)
	svc := &TagService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            obs,
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	color := "#FF0000FF"

	_, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: &color})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
	call := rec.observeCalls[0]
	if call.op != "update" {
		t.Errorf("op: want update, got %q", call.op)
	}
	if call.resource != "tag" {
		t.Errorf("resource: want tag, got %q", call.resource)
	}
}

// --- GetByUUID authz tests ---

func TestTagService_Get_AdminSeesAll(t *testing.T) {
	svc, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	tagQ.grantAdmin(999) // register as admin in mock access-fn simulation

	tag, err := svc.GetByUUID(actorCtx(999), coreQ, tagQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.EntityUUID != tagUUID {
		t.Errorf("uuid mismatch")
	}
}

func TestTagService_Get_OwnerSeesOwn(t *testing.T) {
	svc, coreQ, tagQ, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	_, err := svc.GetByUUID(actorCtx(ownerID), coreQ, tagQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Get_SubjectSeesAsSubject(t *testing.T) {
	svc, coreQ, tagQ, tagUUID, _, subjectID, _ := buildServiceWithTag(t)

	_, err := svc.GetByUUID(actorCtx(subjectID), coreQ, tagQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Get_OtherGetsDenied(t *testing.T) {
	// Row-level read access (owner or subject) is enforced by the Authorizer.
	// Simulate denial via a denyAllAuthz stub to verify the service surfaces
	// the authz error when the Authorizer refuses the read.
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, err := svc.GetByUUID(actorCtx(777), coreQ, tagQ, tagUUID)
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
}

func TestTagService_Get_NotFound(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	// Default EntityResolver policy: returns ErrForbidden when the UUID is
	// not found, masking entity existence (privacy default per Phase E).
	// Resources opting into 404 transparency (e.g. via AllowNotFound("tag"))
	// would return ErrNotFound; tags has not opted in.
	// After Wave 0, entity.ErrForbidden is an alias of the canonical
	// apiresp.ErrForbidden sentinel (re-homed here as ErrForbidden); assert
	// against the re-homed sentinel so this test tracks the canonical set.
	_, err := svc.GetByUUID(actorCtx(1), coreQ, tagQ, uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (UUID-not-found masked as 403), got %v", err)
	}
}

func TestTagService_Get_AuthorizeDenied(t *testing.T) {
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, err := svc.GetByUUID(actorCtx(999), coreQ, tagQ, tagUUID)
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
}

// --- Search authz tests ---

func TestTagService_Search_NoFilter_Returns400Analog(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, err := svc.Search(actorCtx(1), coreQ, tagQ, SearchTagsFilter{}, Pagination{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Search_AdminSeesAll(t *testing.T) {
	svc, coreQ, tagQ, _, ownerID, _, _ := buildServiceWithTag(t)
	tagQ.grantAdmin(999) // register as admin in mock access-fn simulation

	ownerUUID := coreQ.entitiesByID[ownerID].Uuid
	tags, err := svc.Search(actorCtx(999), coreQ, tagQ, SearchTagsFilter{
		OwnerEntityUUID: &ownerUUID,
	}, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("want 1 tag, got %d", len(tags))
	}
}

func TestTagService_Search_NonAdminFilteredToOwned(t *testing.T) {
	svc, coreQ, tagQ, _, ownerID, subjectID, _ := buildServiceWithTag(t)

	ownerUUID := coreQ.entitiesByID[ownerID].Uuid

	// actor is the subject: should see tags where they're subject.
	tags, err := svc.Search(actorCtx(subjectID), coreQ, tagQ, SearchTagsFilter{
		OwnerEntityUUID: &ownerUUID,
	}, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("want 1 tag (as subject), got %d", len(tags))
	}

	// actor is unrelated: should see nothing.
	tags2, err := svc.Search(actorCtx(888), coreQ, tagQ, SearchTagsFilter{
		OwnerEntityUUID: &ownerUUID,
	}, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags2) != 0 {
		t.Errorf("want 0 tags for unrelated actor, got %d", len(tags2))
	}
}

// TestTagService_Search_OwnerFilterEmptyOnMiss and
// TestTagService_Search_SubjectFilterEmptyOnMiss lock in the restored
// pre-task-002 behaviour: a genuine miss on the owner/subject filter UUID
// returns an empty result set (no error), rather than masking to
// ErrForbidden (403). This masking was reverted post-merge per the phase-01
// gate review's security finding — Search's only authorization gate is the
// type-level, filter-independent Authorize(ctx, "list", &tagTypeID) call,
// which is not paired with an instance-scoped Authorize check for the
// specific filter entity (unlike ListBySubject). Masking non-existence via
// ErrForbidden here would have created a previously-absent entity-existence
// oracle spanning the entire entities table, reachable by any actor holding
// the broad "list tags" grant.
func TestTagService_Search_OwnerFilterEmptyOnMiss(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	unknownOwner := uuid.New() // not seeded
	tags, err := svc.Search(actorCtx(1), coreQ, tagQ, SearchTagsFilter{
		OwnerEntityUUID: &unknownOwner,
	}, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("want empty result (owner UUID not found), got %d tags", len(tags))
	}
}

func TestTagService_Search_SubjectFilterEmptyOnMiss(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	unknownSubject := uuid.New() // not seeded
	tags, err := svc.Search(actorCtx(1), coreQ, tagQ, SearchTagsFilter{
		SubjectEntityUUID: &unknownSubject,
	}, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("want empty result (subject UUID not found), got %d tags", len(tags))
	}
}

// --- ListBySubject tests ---

func TestTagService_ListBySubject_SubjectSeesAll(t *testing.T) {
	svc, coreQ, tagQ, _, _, subjectID, _ := buildServiceWithTag(t)

	subjectUUID := coreQ.entitiesByID[subjectID].Uuid
	tags, err := svc.ListBySubject(actorCtx(subjectID), coreQ, tagQ, subjectUUID, nil, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("want 1 tag, got %d", len(tags))
	}
}

func TestTagService_ListBySubject_ThirdPartySeeOnlyOwned(t *testing.T) {
	svc, coreQ, tagQ, _, ownerID, subjectID, _ := buildServiceWithTag(t)

	subjectUUID := coreQ.entitiesByID[subjectID].Uuid

	// Owner is also owner of the tag — should see their own tag.
	tags, err := svc.ListBySubject(actorCtx(ownerID), coreQ, tagQ, subjectUUID, nil, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("want 1 tag (owned), got %d", len(tags))
	}

	// Unrelated actor sees nothing.
	tags2, err := svc.ListBySubject(actorCtx(999), coreQ, tagQ, subjectUUID, nil, Pagination{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags2) != 0 {
		t.Errorf("want 0 tags for unrelated, got %d", len(tags2))
	}
}

func TestTagService_ListBySubject_NotFound(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	// Existence-masking: this list route is scoped to the subject entity, so
	// a genuine miss on that entity masks to ErrForbidden (403), consistent
	// with a direct lookup — not ErrNotFound (404).
	_, err := svc.ListBySubject(actorCtx(1), coreQ, tagQ, uuid.New(), nil, Pagination{})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (subject UUID-not-found masked as 403), got %v", err)
	}
}

// --- UpdateByUUID authz tests ---

func TestTagService_Update_OwnerCanChangeColor(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)
	newColor := "#FF0000FF"

	tag, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: &newColor})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.Color == nil || *tag.Color != newColor {
		t.Errorf("want color %q, got %v", newColor, tag.Color)
	}
}

func TestTagService_Update_AdminCanChangeColor(t *testing.T) {
	svc, coreQ, _, tagUUID, _, _, _ := buildServiceWithTag(t)
	newColor := "#00FF00FF"

	_, err := svc.UpdateByUUID(actorCtx(999), coreQ, tagUUID, UpdateTagInput{Color: &newColor})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Update_SubjectGetsForbidden(t *testing.T) {
	// Row-level ownership for update (owner-only, not subject) is enforced by
	// the Authorizer. Simulate denial via a denyAllAuthz stub.
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: ErrForbidden},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	newColor := "#0000FFFF"

	_, err := svc.UpdateByUUID(actorCtx(1), coreQ, tagUUID, UpdateTagInput{Color: &newColor})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestTagService_Update_StrangerGets404(t *testing.T) {
	// A tag that doesn't exist masks to ErrForbidden (403) regardless of
	// actor — entityResolver.Resolve is called pre-tx and tags has not
	// opted the "tag" slug into AllowNotFound.
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)
	newColor := "#FFFFFFFF"

	_, err := svc.UpdateByUUID(actorCtx(888), coreQ, uuid.New(), UpdateTagInput{Color: &newColor})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (UUID-not-found masked as 403), got %v", err)
	}
}

func TestTagService_Update_InvalidColor(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)
	bad := "notacolor"

	_, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: &bad})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Update_NilColorClears(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	tag, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.Color != nil {
		t.Errorf("want nil color after clear, got %v", tag.Color)
	}
}

// --- UpdateTagValue tests ---

func TestTagService_UpdateTagValue_HappyPath(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	tag, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: "new-value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.Value != "new-value" {
		t.Errorf("want value %q, got %q", "new-value", tag.Value)
	}
}

func TestTagService_UpdateTagValue_EmptyValue(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	_, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: ""})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestTagService_UpdateTagValue_WhitespaceOnly(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	_, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: "   "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput for whitespace-only value, got %v", err)
	}
}

func TestTagService_UpdateTagValue_TooLong(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	long := string(make([]byte, 513))
	for i := range long {
		long = long[:i] + "a" + long[i+1:]
	}

	_, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: long})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput for value exceeding 512 chars, got %v", err)
	}
}

func TestTagService_UpdateTagValue_NotFound(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	// Existence-masking: entityResolver.Resolve is called pre-tx; a genuine
	// miss masks to ErrForbidden (403), not ErrNotFound (404).
	_, err := svc.UpdateTagValue(actorCtx(1), coreQ, uuid.New(), UpdateTagValueInput{Value: "v"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (UUID-not-found masked as 403), got %v", err)
	}
}

func TestTagService_UpdateTagValue_AuthorizeDenied(t *testing.T) {
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	authzErr := errors.New("not authorized")
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: authzErr},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, err := svc.UpdateTagValue(actorCtx(999), coreQ, tagUUID, UpdateTagValueInput{Value: "v"})
	if !errors.Is(err, authzErr) {
		t.Errorf("want authz error, got %v", err)
	}
}

func TestTagService_UpdateTagValue_ObserverCalled(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, _ := buildServiceWithTag(t)
	rec := &recordingObserver{}
	obs := observer.NewObserverGroup(rec)
	svc := &TagService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            obs,
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: "observed-value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
	call := rec.observeCalls[0]
	if call.op != "update" {
		t.Errorf("op: want update, got %q", call.op)
	}
	if call.resource != "tag" {
		t.Errorf("resource: want tag, got %q", call.resource)
	}
}

// --- DeleteByUUID authz tests ---

func TestTagService_Delete_OwnerOK(t *testing.T) {
	svc, coreQ, _, tagUUID, ownerID, _, _ := buildServiceWithTag(t)

	err := svc.DeleteByUUID(actorCtx(ownerID), coreQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Delete_AdminOK(t *testing.T) {
	svc, coreQ, _, tagUUID, _, _, _ := buildServiceWithTag(t)

	err := svc.DeleteByUUID(actorCtx(999), coreQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTagService_Delete_SubjectGetsForbidden(t *testing.T) {
	// Row-level ownership for delete (owner-only, not subject) is enforced by
	// the Authorizer. Simulate denial via a denyAllAuthz stub.
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	svc := &TagService{
		db:             newFakeDB(),
		az:             denyAllAuthz{err: ErrForbidden},
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	err := svc.DeleteByUUID(actorCtx(1), coreQ, tagUUID)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden, got %v", err)
	}
}

func TestTagService_Delete_StrangerGets404(t *testing.T) {
	// A tag that doesn't exist masks to ErrForbidden (403) regardless of
	// actor — entityResolver.Resolve is called pre-tx and tags has not
	// opted the "tag" slug into AllowNotFound.
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	err := svc.DeleteByUUID(actorCtx(888), coreQ, uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (UUID-not-found masked as 403), got %v", err)
	}
}

func TestTagService_Delete_NotFound(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	err := svc.DeleteByUUID(actorCtx(1), coreQ, uuid.New())
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden (UUID-not-found masked as 403), got %v", err)
	}
}

func TestTagService_Delete_ObserverCalled(t *testing.T) {
	_, coreQ, tagQ, tagUUID, _, _, _ := buildServiceWithTag(t)
	rec := &recordingObserver{}
	obs := observer.NewObserverGroup(rec)
	svc := &TagService{
		db:             newFakeDB(),
		az:             allowAllAuthz{},
		obs:            obs,
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	err := svc.DeleteByUUID(actorCtx(999), coreQ, tagUUID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.observeCalls) != 1 {
		t.Fatalf("expected 1 observe call, got %d", len(rec.observeCalls))
	}
	call := rec.observeCalls[0]
	if call.op != "delete" {
		t.Errorf("op: want delete, got %q", call.op)
	}
	if call.resource != "tag" {
		t.Errorf("resource: want tag, got %q", call.resource)
	}
}

// --- Authorize target-shape regression tests (followup 19B5) ---
//
// allowAllAuthz/denyAllAuthz (used throughout this file) ignore the target
// argument entirely, so they cannot catch a call site that passes the wrong
// target shape — which is exactly what let the bug these tests guard
// against ship untested against a real (non-stub) Authorizer. authzFunc
// (mock_test.go) captures op/target so these tests assert the actual value
// passed to Authorize. Mirrors mod-tasks/api/service/task_test.go's
// TestTaskService_Create_AuthorizesAgainstActorEntityID family.

func TestTagService_Create_AuthorizesAgainstActorEntityID(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()

	var gotOp string
	var gotTarget *int64
	recAuthz := authzFunc(func(_ context.Context, op string, target *int64) error {
		gotOp = op
		gotTarget = target
		return nil
	})
	svc := &TagService{
		db:             newFakeDB(),
		az:             recAuthz,
		obs:            observer.NewObserverGroup(),
		resolver:       mustResolver(coreQ),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	_, actorID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")

	if _, err := svc.Create(actorCtx(actorID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "label",
		Value:             "urgent",
	}); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if gotOp != "create" {
		t.Errorf("Authorize op = %q, want %q", gotOp, "create")
	}
	if gotTarget == nil || *gotTarget != actorID {
		t.Errorf("Authorize target = %v, want &actorID(%d) — Create must authorize against the actor's own entity ID, not the tag type ID", gotTarget, actorID)
	}
}

func TestTagService_Search_AuthorizesAgainstActorEntityID(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()

	var gotOp string
	var gotTarget *int64
	recAuthz := authzFunc(func(_ context.Context, op string, target *int64) error {
		gotOp = op
		gotTarget = target
		return nil
	})
	svc := &TagService{
		db:             newFakeDB(),
		az:             recAuthz,
		opRes:          newStubOpResolver(),
		obs:            observer.NewObserverGroup(),
		resolver:       mustResolver(coreQ),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	ownerUUID, actorID := coreQ.seedEntity("natural_person")

	if _, err := svc.Search(actorCtx(actorID), coreQ, tagQ, SearchTagsFilter{OwnerEntityUUID: &ownerUUID}, Pagination{}); err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if gotOp != "list" {
		t.Errorf("Authorize op = %q, want %q", gotOp, "list")
	}
	if gotTarget == nil || *gotTarget != actorID {
		t.Errorf("Authorize target = %v, want &actorID(%d) — Search must authorize against the actor's own entity ID, not the tag type ID", gotTarget, actorID)
	}
}

func TestTagService_UpdateByUUID_AuthorizesAgainstTagEntityID(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, entityID := buildServiceWithTag(t)

	var gotOp string
	var gotTarget *int64
	recAuthz := authzFunc(func(_ context.Context, op string, target *int64) error {
		gotOp = op
		gotTarget = target
		return nil
	})
	svc := &TagService{
		db:             newFakeDB(),
		az:             recAuthz,
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	color := "#FF0000FF"

	if _, err := svc.UpdateByUUID(actorCtx(ownerID), coreQ, tagUUID, UpdateTagInput{Color: &color}); err != nil {
		t.Fatalf("UpdateByUUID: unexpected error: %v", err)
	}
	if gotOp != "update" {
		t.Errorf("Authorize op = %q, want %q", gotOp, "update")
	}
	if gotTarget == nil || *gotTarget != entityID {
		t.Errorf("Authorize target = %v, want &entityID(%d) — UpdateByUUID must authorize against the tag's own entity ID, not nil", gotTarget, entityID)
	}
}

func TestTagService_UpdateTagValue_AuthorizesAgainstTagEntityID(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, entityID := buildServiceWithTag(t)

	var gotOp string
	var gotTarget *int64
	recAuthz := authzFunc(func(_ context.Context, op string, target *int64) error {
		gotOp = op
		gotTarget = target
		return nil
	})
	svc := &TagService{
		db:             newFakeDB(),
		az:             recAuthz,
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	if _, err := svc.UpdateTagValue(actorCtx(ownerID), coreQ, tagUUID, UpdateTagValueInput{Value: "new-value"}); err != nil {
		t.Fatalf("UpdateTagValue: unexpected error: %v", err)
	}
	if gotOp != "update" {
		t.Errorf("Authorize op = %q, want %q", gotOp, "update")
	}
	if gotTarget == nil || *gotTarget != entityID {
		t.Errorf("Authorize target = %v, want &entityID(%d) — UpdateTagValue must authorize against the tag's own entity ID, not nil", gotTarget, entityID)
	}
}

func TestTagService_DeleteByUUID_AuthorizesAgainstTagEntityID(t *testing.T) {
	_, coreQ, tagQ, tagUUID, ownerID, _, entityID := buildServiceWithTag(t)

	var gotOp string
	var gotTarget *int64
	recAuthz := authzFunc(func(_ context.Context, op string, target *int64) error {
		gotOp = op
		gotTarget = target
		return nil
	})
	svc := &TagService{
		db:             newFakeDB(),
		az:             recAuthz,
		obs:            observer.NewObserverGroup(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}

	if err := svc.DeleteByUUID(actorCtx(ownerID), coreQ, tagUUID); err != nil {
		t.Fatalf("DeleteByUUID: unexpected error: %v", err)
	}
	if gotOp != "delete" {
		t.Errorf("Authorize op = %q, want %q", gotOp, "delete")
	}
	if gotTarget == nil || *gotTarget != entityID {
		t.Errorf("Authorize target = %v, want &entityID(%d) — DeleteByUUID must authorize against the tag's own entity ID, not nil", gotTarget, entityID)
	}
}

// --- Missing-actor is 401 (ErrUnauthenticated), not 403 (ErrForbidden) ---
//
// Mirrors mod-tasks/api/service/task_test.go's TestTaskService_Create_MissingActor:
// no actor on context means the caller was never authenticated in the first
// place, distinct from an authenticated actor being denied by the Authorizer.

func TestTagService_Create_MissingActor(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, err := svc.Create(context.Background(), coreQ, CreateTagInput{
		SubjectEntityUUID: uuid.New(),
		Purpose:           "label",
		Value:             "v",
	})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("want ErrUnauthenticated, got %v", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Errorf("must not also be ErrForbidden (401 and 403 are distinct, not collapsed), got %v", err)
	}
}

func TestTagService_Search_MissingActor(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	ownerUUID, _ := coreQ.seedEntity("natural_person")
	_, err := svc.Search(context.Background(), coreQ, tagQ, SearchTagsFilter{OwnerEntityUUID: &ownerUUID}, Pagination{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("want ErrUnauthenticated, got %v", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Errorf("must not also be ErrForbidden (401 and 403 are distinct, not collapsed), got %v", err)
	}
}

func TestTagService_ListBySubject_MissingActor(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	subjectUUID, _ := coreQ.seedEntity("natural_person")
	_, err := svc.ListBySubject(context.Background(), coreQ, tagQ, subjectUUID, nil, Pagination{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("want ErrUnauthenticated, got %v", err)
	}
	if errors.Is(err, ErrForbidden) {
		t.Errorf("must not also be ErrForbidden (401 and 403 are distinct, not collapsed), got %v", err)
	}
}

// --- One-of-domain conflict tests (phase-02 task 002, Requirement 6) ---
//
// These exercise the mock's simulated one-of-domain gate (mockTagQuerier.
// CreateTag, mock_test.go), which mirrors the real tags_enforce_one_of_domain
// trigger's SQLSTATE-23505-on-conflict behavior
// (model/migrations/0205_tags_one_of_domain.sql) via a *pgconn.PgError with
// Code: pgUniqueViolation — proving TagService.Create's existing
// errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation classification
// (tag.go) picks it up without any live Postgres connection. The real
// trigger itself is proven separately by the integration test in
// tag_one_of_domain_integration_test.go.

func TestTagService_Create_OneOfDomainConflict(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	tagQ.seedPurposePolicy("priority", true)

	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, subjectID := coreQ.seedEntity("natural_person")

	// Seed an existing "priority" tag for (owner, subject).
	_, existingEntityID := coreQ.seedEntity("tag")
	tagQ.seedTag(existingEntityID, ownerID, subjectID, "priority", "low", nil)

	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "priority",
		Value:             "urgent",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

// TestTagService_Create_OneOfDomainConflict_DifferentPurposeSucceeds proves
// the simulated gate is conditional on purpose, not a blanket (owner,
// subject) uniqueness rule: a second tag for the same (owner, subject) but a
// different purpose — one with no policy row, defaulting to
// one_of_domain=false — must succeed even though a one-of-domain "priority"
// tag already exists for that same (owner, subject) pair.
func TestTagService_Create_OneOfDomainConflict_DifferentPurposeSucceeds(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	tagQ.seedPurposePolicy("priority", true)

	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, subjectID := coreQ.seedEntity("natural_person")

	_, existingEntityID := coreQ.seedEntity("tag")
	tagQ.seedTag(existingEntityID, ownerID, subjectID, "priority", "low", nil)

	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "label", // no policy row for "label" -> defaults to false
		Value:             "urgent",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
}

// --- OneOfDomain field round-trip tests (phase-boundary review followup) ---
//
// The tests above only assert on the returned error from Create; they never
// inspect the returned service.Tag's OneOfDomain field itself. That leaves a
// gap: nothing catches a regression where one of the hydrateTag /
// hydrateTagFromSearchRow / hydrateTagFromListRow call sites in tag.go
// silently drops or swaps its oneOfDomain argument. These tests assert
// OneOfDomain directly on the returned Tag for both Create and GetByUUID —
// GetByUUID via tagFromUUIDRow is the highest-risk call site, since its
// OneOfDomain value round-trips through an extra row-shape conversion.

func TestTagService_Create_OneOfDomainTrue_SetsOneOfDomainField(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	tagQ.seedPurposePolicy("priority", true)

	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")

	got, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "priority",
		Value:             "urgent",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !got.OneOfDomain {
		t.Errorf("OneOfDomain = false, want true (purpose %q has a one-of-domain policy)", "priority")
	}
}

func TestTagService_Create_OneOfDomainFalse_NoPolicySetsOneOfDomainField(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")

	got, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID,
		Purpose:           "label", // no policy row for "label" -> defaults to false
		Value:             "urgent",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.OneOfDomain {
		t.Errorf("OneOfDomain = true, want false (purpose %q has no policy row)", "label")
	}
}

func TestTagService_GetByUUID_OneOfDomainTrue_SetsOneOfDomainField(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	tagQ.seedPurposePolicy("priority", true)

	_, ownerID := coreQ.seedEntity("natural_person")
	_, subjectID := coreQ.seedEntity("natural_person")
	_, entityID := coreQ.seedEntity("tag")
	tagUUID, _ := tagQ.seedTag(entityID, ownerID, subjectID, "priority", "urgent", nil)
	coreQ.entities[tagUUID] = coreQ.entitiesByID[entityID]

	tag, err := svc.GetByUUID(actorCtx(ownerID), coreQ, tagQ, tagUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if !tag.OneOfDomain {
		t.Errorf("OneOfDomain = false, want true (purpose %q has a one-of-domain policy); GetByUUID round-trips OneOfDomain through tagFromUUIDRow", "priority")
	}
}

func TestTagService_GetByUUID_OneOfDomainFalse_NoPolicySetsOneOfDomainField(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc, _ := buildService(coreQ, tagQ)

	_, ownerID := coreQ.seedEntity("natural_person")
	_, subjectID := coreQ.seedEntity("natural_person")
	_, entityID := coreQ.seedEntity("tag")
	tagUUID, _ := tagQ.seedTag(entityID, ownerID, subjectID, "label", "urgent", nil)
	coreQ.entities[tagUUID] = coreQ.entitiesByID[entityID]

	tag, err := svc.GetByUUID(actorCtx(ownerID), coreQ, tagQ, tagUUID)
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if tag.OneOfDomain {
		t.Errorf("OneOfDomain = true, want false (purpose %q has no policy row); GetByUUID round-trips OneOfDomain through tagFromUUIDRow", "label")
	}
}

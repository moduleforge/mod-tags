package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// tgSudoService builds a TagService whose Authorize calls are counted.
func tgSudoService(coreQ *mockCoreQuerier, tagQ *mockTagQuerier, calls *atomic.Int32) *TagService {
	return &TagService{
		db: newFakeDB(),
		az: authzFunc(func(_ context.Context, _ string, _ *int64) error {
			calls.Add(1)
			return nil
		}),
		opRes:          newStubOpResolver(),
		obs:            observer.NewObserverGroup(),
		resolver:       mustResolver(coreQ),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
}

func tgSudoCtx() context.Context {
	return opctx.WithSudoActor(context.Background(), 99)
}

func TestTagService_tgSudo_Create(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	var calls atomic.Int32
	svc := tgSudoService(coreQ, tagQ, &calls)
	subjectUUID, _ := coreQ.seedEntity("natural_person")
	in := CreateTagInput{SubjectEntityUUID: subjectUUID, Purpose: "label", Value: "v"}

	if _, err := svc.Create(context.Background(), coreQ, in); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no identity: want ErrUnauthenticated, got %v", err)
	}
	if _, err := svc.Create(tgSudoCtx(), coreQ, in); !errors.Is(err, ErrForbidden) {
		t.Errorf("sudo-only: want ErrForbidden, got %v", err)
	}
	if len(tagQ.tags) != 0 {
		t.Error("expected no tag written for sudo-only or unauthenticated caller")
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("Authorize called %d times, want 0", n)
	}

	// Regular actor unchanged: owner is the regular actor.
	_, ownerID := coreQ.seedEntity("natural_person")
	if _, err := svc.Create(actorCtx(ownerID), coreQ, in); err != nil {
		t.Fatalf("regular actor: %v", err)
	}
	if calls.Load() == 0 {
		t.Error("regular actor: expected Authorize to be called")
	}
}

func TestTagService_tgSudo_Search(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	var calls atomic.Int32
	svc := tgSudoService(coreQ, tagQ, &calls)
	ownerUUID, ownerID := coreQ.seedEntity("natural_person")
	filter := SearchTagsFilter{OwnerEntityUUID: &ownerUUID}

	if _, err := svc.Search(context.Background(), coreQ, tagQ, filter, Pagination{}); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no identity: want ErrUnauthenticated, got %v", err)
	}
	if _, err := svc.Search(tgSudoCtx(), coreQ, tagQ, filter, Pagination{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("sudo-only: want ErrForbidden, got %v", err)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("Authorize called %d times, want 0", n)
	}
	if _, err := svc.Search(actorCtx(ownerID), coreQ, tagQ, filter, Pagination{}); errors.Is(err, ErrForbidden) || errors.Is(err, ErrUnauthenticated) {
		t.Errorf("regular actor: unexpected %v", err)
	}
	if calls.Load() == 0 {
		t.Error("regular actor: expected Authorize to be called")
	}
}

func TestTagService_tgSudo_ListBySubject(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	var calls atomic.Int32
	svc := tgSudoService(coreQ, tagQ, &calls)
	subjectUUID, subjectID := coreQ.seedEntity("natural_person")

	if _, err := svc.ListBySubject(context.Background(), coreQ, tagQ, subjectUUID, nil, Pagination{}); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("no identity: want ErrUnauthenticated, got %v", err)
	}
	if _, err := svc.ListBySubject(tgSudoCtx(), coreQ, tagQ, subjectUUID, nil, Pagination{}); !errors.Is(err, ErrForbidden) {
		t.Errorf("sudo-only: want ErrForbidden, got %v", err)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("Authorize called %d times, want 0", n)
	}
	if _, err := svc.ListBySubject(actorCtx(subjectID), coreQ, tagQ, subjectUUID, nil, Pagination{}); err != nil {
		t.Errorf("regular actor: %v", err)
	}
	if calls.Load() == 0 {
		t.Error("regular actor: expected Authorize to be called")
	}
}

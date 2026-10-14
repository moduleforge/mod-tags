package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// tgShapeService builds a TagService whose authorizer denies every call and
// counts how many times it was invoked.
func tgShapeService(calls *int) (*TagService, *mockCoreQuerier, *mockTagQuerier) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	deny := errors.New("denied")
	svc := &TagService{
		db: newFakeDB(),
		az: authzFunc(func(_ context.Context, _ string, _ *int64) error {
			*calls++
			return deny
		}),
		obs:            observer.NewObserverGroup(),
		resolver:       mustResolver(coreQ),
		opRes:          newStubOpResolver(),
		entityResolver: entity.NewResolver(),
		newCoreQuerier: func(_ pgx.Tx) coredb.Querier { return coreQ },
		newTagQuerier:  func(_ pgx.Tx) tagsdb.Querier { return tagQ },
	}
	return svc, coreQ, tagQ
}

func TestTgShape_Create_StatelessValidationBeforeAuthorize(t *testing.T) {
	badColor := "#nothex"
	long := strings.Repeat("a", 513)
	cases := []struct {
		name string
		in   CreateTagInput
	}{
		{"blank purpose", CreateTagInput{Purpose: "  ", Value: "v"}},
		{"blank value", CreateTagInput{Purpose: "p", Value: ""}},
		{"long purpose", CreateTagInput{Purpose: long, Value: "v"}},
		{"long value", CreateTagInput{Purpose: "p", Value: long}},
		{"bad color", CreateTagInput{Purpose: "p", Value: "v", Color: &badColor}},
		{"NUL purpose", CreateTagInput{Purpose: "a\x00b", Value: "v"}},
		{"NUL value", CreateTagInput{Purpose: "p", Value: "a\x00b"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var calls int
			svc, coreQ, _ := tgShapeService(&calls)
			c.in.SubjectEntityUUID = uuid.New()
			_, err := svc.Create(actorCtx(1), coreQ, c.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("want ErrInvalidInput, got %v", err)
			}
			if calls != 0 {
				t.Errorf("Authorize called %d times, want 0", calls)
			}
		})
	}
}

func TestTgShape_Create_NULHasDetail(t *testing.T) {
	var calls int
	svc, coreQ, _ := tgShapeService(&calls)
	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{SubjectEntityUUID: uuid.New(), Purpose: "a\x00b", Value: "v"})
	if !errors.Is(err, apiresp.ErrInvalidInput) {
		t.Fatalf("want invalid input, got %v", err)
	}
	rec := httptest.NewRecorder()
	apiresp.WriteError(rec, httptest.NewRequest(http.MethodPost, "/tags", nil), err)
	var env apiresp.Envelope
	if jerr := json.Unmarshal(rec.Body.Bytes(), &env); jerr != nil {
		t.Fatal(jerr)
	}
	if rec.Code != http.StatusBadRequest || len(env.Error.Details) != 1 ||
		env.Error.Details[0].Code != "tags.nul_character" || env.Error.Details[0].Field != "purpose" {
		t.Errorf("got status %d body %s, want 400 with tags.nul_character on purpose", rec.Code, rec.Body.String())
	}
}

func TestTgShape_Search_NoFilterBeforeAuthorize(t *testing.T) {
	var calls int
	svc, coreQ, tagQ := tgShapeService(&calls)
	_, err := svc.Search(actorCtx(1), coreQ, tagQ, SearchTagsFilter{}, Pagination{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if calls != 0 {
		t.Errorf("Authorize called %d times, want 0", calls)
	}
}

func TestTgShape_Search_NULFilterBeforeAuthorize(t *testing.T) {
	var calls int
	svc, coreQ, tagQ := tgShapeService(&calls)
	owner := uuid.New()
	nul := "a\x00b"
	for _, f := range []SearchTagsFilter{
		{OwnerEntityUUID: &owner, Purpose: &nul},
		{OwnerEntityUUID: &owner, Value: &nul},
	} {
		_, err := svc.Search(actorCtx(1), coreQ, tagQ, f, Pagination{})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("want ErrInvalidInput, got %v", err)
		}
	}
	if calls != 0 {
		t.Errorf("Authorize called %d times, want 0", calls)
	}
}

// Positive control: a valid request does reach the (denying) authorizer.
func TestTgShape_Create_ValidReachesAuthorize(t *testing.T) {
	var calls int
	svc, coreQ, _ := tgShapeService(&calls)
	_, err := svc.Create(actorCtx(1), coreQ, CreateTagInput{SubjectEntityUUID: uuid.New(), Purpose: "p", Value: "v"})
	if err == nil || errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want the authorizer error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("Authorize called %d times, want 1", calls)
	}
}

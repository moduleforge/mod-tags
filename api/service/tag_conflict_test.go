package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/moduleforge/core-api/apiresp"
)

// tgConfTriggerErr is the error the tags_enforce_one_of_domain trigger
// (model/migrations/0205_tags_one_of_domain.sql) surfaces: SQLSTATE 23505
// raised with no constraint name.
func tgConfTriggerErr() error {
	return &pgconn.PgError{Code: "23505"}
}

// A 23505 from CreateTag, after a successful authorize, is a 409 conflict.
func TestTagService_tgConf_Create23505IsConflictAfterAuthorize(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")
	tagQ.createErr = tgConfTriggerErr()

	authorized := 0
	svc, _ := buildService(coreQ, tagQ)
	svc.az = authzFunc(func(_ context.Context, op string, _ *int64) error {
		if op == "create" {
			authorized++
		}
		return nil
	})

	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID, Purpose: "priority", Value: "high",
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	if authorized != 1 {
		t.Errorf("Authorize(create) calls = %d, want 1 (conflict only after authorization)", authorized)
	}
}

// A non-unique-violation insert error stays a 500 (not mapped to a conflict).
func TestTagService_tgConf_CreateOtherDBErrorIsNotConflict(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")
	tagQ.createErr = &pgconn.PgError{Code: "23503"}
	svc, _ := buildService(coreQ, tagQ)

	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID, Purpose: "p", Value: "v",
	})
	if err == nil || errors.Is(err, ErrConflict) {
		t.Fatalf("want a non-conflict error, got %v", err)
	}
}

// A denied caller gets the authorization error, never the conflict: the
// insert is not attempted, so the trigger cannot fire for them.
func TestTagService_tgConf_DeniedCallerNeverSeesConflict(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")
	tagQ.createErr = tgConfTriggerErr() // would yield ErrConflict if reached
	svc, _ := buildService(coreQ, tagQ)
	svc.az = denyAllAuthz{err: ErrForbidden}

	_, err := svc.Create(actorCtx(ownerID), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectUUID, Purpose: "priority", Value: "high",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
	if errors.Is(err, ErrConflict) {
		t.Errorf("denied caller must not see a conflict, got %v", err)
	}
	if len(tagQ.tags) != 0 {
		t.Error("insert must not be attempted for a denied caller")
	}
}

// ListBySubject rejects a NUL purpose filter with a 400 before any lookup or
// authorization.
func TestTagService_tgConf_ListBySubjectNULPurposeIs400(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	_, ownerID := coreQ.seedEntity("natural_person")
	subjectUUID, _ := coreQ.seedEntity("natural_person")
	svc, _ := buildService(coreQ, tagQ)
	called := false
	svc.az = authzFunc(func(_ context.Context, _ string, _ *int64) error {
		called = true
		return ErrForbidden
	})

	bad := "a\x00b"
	_, err := svc.ListBySubject(actorCtx(ownerID), coreQ, tagQ, subjectUUID, &bad, Pagination{})
	if !errors.Is(err, apiresp.ErrInvalidInput) {
		t.Fatalf("want invalid input, got %v", err)
	}
	if called {
		t.Fatal("validation must precede authorization")
	}
	rec := httptest.NewRecorder()
	apiresp.WriteError(rec, httptest.NewRequest(http.MethodGet, "/entities/x/tags", nil), err)
	var env apiresp.Envelope
	if jerr := json.Unmarshal(rec.Body.Bytes(), &env); jerr != nil {
		t.Fatal(jerr)
	}
	if rec.Code != http.StatusBadRequest || len(env.Error.Details) != 1 ||
		env.Error.Details[0].Code != "tags.nul_character" || env.Error.Details[0].Field != "purpose" {
		t.Errorf("got status %d body %s, want 400 with tags.nul_character on purpose", rec.Code, rec.Body.String())
	}

	// Unknown subject with a NUL filter is still 400, not a masked 403.
	_, err = svc.ListBySubject(actorCtx(ownerID), coreQ, tagQ, uuid.New(), &bad, Pagination{})
	if err == nil || errors.Is(err, ErrForbidden) {
		t.Errorf("want 400 for NUL purpose even on unknown subject, got %v", err)
	}
}

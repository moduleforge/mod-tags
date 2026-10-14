package service_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/moduleforge/core-api/apiresp"
	"github.com/moduleforge/core-api/apiresp/ordertest"
	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/tags-api/httpapi"
	"github.com/moduleforge/tags-api/service"
)

// orderOneOfDomainPurpose is the seeded purpose whose policy is one-of-domain.
const orderOneOfDomainPurpose = "exclusive"

// orderAuthorizer is the stub authz.Authorizer for the order tests. The owner
// may act on its own entity, the subject entity (for listing) and the seeded
// tag; the stranger and every other target, including the target-0 probe,
// are denied with a plain forbidden error (common rule 5: never errors
// loudly, never grants). It counts every call so tests can assert a route
// never authorizes.
type orderAuthorizer struct {
	ownerID int64
	allowed map[int64]bool
	calls   atomic.Int64
}

func (a *orderAuthorizer) Authorize(ctx context.Context, _ string, target *int64) error {
	a.calls.Add(1)
	actor, ok := opctx.ActorEntityID(ctx)
	if !ok || actor != a.ownerID || target == nil || *target == 0 || !a.allowed[*target] {
		return apiresp.ErrForbidden
	}
	return nil
}

var _ authz.Authorizer = (*orderAuthorizer)(nil)

// orderEnv is one fully seeded fixture plus the real router over it.
type orderEnv struct {
	router http.Handler
	az     *orderAuthorizer

	owner, stranger        ordertest.Identity
	ownerID, strangerID    int64
	subjectUUID            uuid.UUID
	tagUUID                uuid.UUID
	missingUUID            uuid.UUID // well-formed, absent
	missingSubjectUUID     uuid.UUID
	tagPath, missingTag    string
	subjectTagsPath        string
	missingSubjectTagsPath string
}

// orderRouter seeds an owner, a stranger, a subject entity, one tag (owner ->
// subject, purpose "label"), a one-of-domain purpose policy plus an existing
// owner tag under it on the subject, and one global tag template, then
// builds the real httpapi.NewRouter over the real services. No identity
// middleware is installed, so the identity ordertest places on the request
// context reaches RequireAuth and the authorizer unchanged.
func orderRouter() *orderEnv {
	coreQ := service.OrderNewCoreQuerier()
	tagQ := service.OrderNewTagQuerier()

	_, ownerID := service.OrderSeedEntity(coreQ, "natural_person")
	_, strangerID := service.OrderSeedEntity(coreQ, "natural_person")
	subjectUUID, subjectID := service.OrderSeedEntity(coreQ, "natural_person")
	tagUUID, tagEntityID := service.OrderSeedTag(coreQ, tagQ, ownerID, subjectID, "label", "urgent")

	service.OrderSeedPurposePolicy(tagQ, orderOneOfDomainPurpose, true)
	service.OrderSeedTag(coreQ, tagQ, ownerID, subjectID, orderOneOfDomainPurpose, "first")
	service.OrderSeedGlobalTemplate(tagQ, "label", "urgent", "Urgent")

	az := &orderAuthorizer{
		ownerID: ownerID,
		allowed: map[int64]bool{ownerID: true, subjectID: true, tagEntityID: true},
	}
	svcs := service.OrderNewServices(coreQ, tagQ, service.OrderNewFakeDB(), az,
		service.OrderNewOpResolver(), observer.NewObserverGroup())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpapi.NewRouter(httpapi.NewDeps(coreQ, svcs, logger))

	missing := uuid.New()
	missingSubject := uuid.New()
	return &orderEnv{
		router:                 router,
		az:                     az,
		owner:                  ordertest.Actor(ownerID),
		stranger:               ordertest.Actor(strangerID),
		ownerID:                ownerID,
		strangerID:             strangerID,
		subjectUUID:            subjectUUID,
		tagUUID:                tagUUID,
		missingUUID:            missing,
		missingSubjectUUID:     missingSubject,
		tagPath:                "/tags/" + tagUUID.String(),
		missingTag:             "/tags/" + missing.String(),
		subjectTagsPath:        "/entities/" + subjectUUID.String() + "/tags",
		missingSubjectTagsPath: "/entities/" + missingSubject.String() + "/tags",
	}
}

// orderDo serves one request as id and returns the recorder.
func orderDo(t *testing.T, h http.Handler, id ordertest.Identity, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(id(req.Context()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// orderExpect asserts the status of one request.
func orderExpect(t *testing.T, h http.Handler, id ordertest.Identity, method, path, body string, want int) {
	t.Helper()
	rec := orderDo(t, h, id, method, path, body)
	if rec.Code != want {
		t.Fatalf("%s %s: status = %d, want %d; body: %s", method, path, rec.Code, want, rec.Body.String())
	}
}

// TestOrderAuthorizer_Target0Denies pins the stub contract from the
// conformance rules: the target-0 probe is denied, not errored.
func TestOrderAuthorizer_Target0Denies(t *testing.T) {
	env := orderRouter()
	var zero int64
	ctx := opctx.WithActor(context.Background(), env.ownerID)
	if err := env.az.Authorize(ctx, "read", &zero); err != apiresp.ErrForbidden {
		t.Fatalf("target 0 authorize = %v, want apiresp.ErrForbidden", err)
	}
}

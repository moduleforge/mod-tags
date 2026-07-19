package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestTagTemplateService_List_MissingPurpose(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	_, err := svc.List(context.Background(), coreQ, tagQ, "", nil)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err: got %v, want ErrInvalidInput", err)
	}
}

func TestTagTemplateService_List_PurposeOnly_GlobalsOnly(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	appUUID, appID := coreQ.seedEntity("tag") // type slug irrelevant to List; any seeded entity works
	color := "#FF0000FF"
	tagQ.seedTemplate("status", "active", "Active", &color, 0, 0, uuid.Nil)   // global
	tagQ.seedTemplate("status", "blocked", "Blocked", nil, 1, appID, appUUID) // scoped to appID
	tagQ.seedTemplate("other-purpose", "x", "X", nil, 0, 0, uuid.Nil)         // different purpose, excluded

	got, err := svc.List(context.Background(), coreQ, tagQ, "status", nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got): got %d, want 1 (globals only); got=%+v", len(got), got)
	}
	if got[0].Value != "active" || got[0].Scope != nil {
		t.Errorf("got[0]: got %+v, want value=active scope=nil", got[0])
	}
	if got[0].Color == nil || *got[0].Color != color {
		t.Errorf("got[0].Color: got %v, want %q", got[0].Color, color)
	}
}

func TestTagTemplateService_List_ValidScope_GlobalsPlusScoped(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	appUUID, appID := coreQ.seedEntity("tag")
	otherAppUUID, otherAppID := coreQ.seedEntity("tag")

	tagQ.seedTemplate("status", "active", "Active", nil, 0, 0, uuid.Nil)                // global
	tagQ.seedTemplate("status", "custom", "Custom", nil, 1, appID, appUUID)             // scoped to appID
	tagQ.seedTemplate("status", "other-app", "Other", nil, 2, otherAppID, otherAppUUID) // scoped to a different app

	got, err := svc.List(context.Background(), coreQ, tagQ, "status", &appUUID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got): got %d, want 2 (global + own scope); got=%+v", len(got), got)
	}

	var sawGlobal, sawScoped bool
	for _, tt := range got {
		switch tt.Value {
		case "active":
			if tt.Scope != nil {
				t.Errorf("global row: got scope %v, want nil", tt.Scope)
			}
			sawGlobal = true
		case "custom":
			if tt.Scope == nil || *tt.Scope != appUUID {
				t.Errorf("scoped row: got scope %v, want %v", tt.Scope, appUUID)
			}
			sawScoped = true
		default:
			t.Errorf("unexpected row in result: %+v", tt)
		}
	}
	if !sawGlobal || !sawScoped {
		t.Errorf("result missing expected rows: sawGlobal=%v sawScoped=%v", sawGlobal, sawScoped)
	}
}

func TestTagTemplateService_List_UnknownScope_GlobalsOnly(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	tagQ.seedTemplate("status", "active", "Active", nil, 0, 0, uuid.Nil)

	unknownScope := uuid.New() // well-formed, never seeded in coreQ
	got, err := svc.List(context.Background(), coreQ, tagQ, "status", &unknownScope)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got): got %d, want 1 (forgiving open-read → globals only); got=%+v", len(got), got)
	}
	if got[0].Scope != nil {
		t.Errorf("got[0].Scope: got %v, want nil", got[0].Scope)
	}
}

func TestTagTemplateService_List_QueryError(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	tagQ.listTemplatesErr = errors.New("boom")
	svc := &TagTemplateService{}

	_, err := svc.List(context.Background(), coreQ, tagQ, "status", nil)
	if err == nil {
		t.Fatal("List: got nil error, want non-nil")
	}
}

func TestTagTemplateService_List_TrimsPurpose(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	tagQ.seedTemplate("status", "active", "Active", nil, 0, 0, uuid.Nil)

	got, err := svc.List(context.Background(), coreQ, tagQ, "  status  ", nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got): got %d, want 1", len(got))
	}
}

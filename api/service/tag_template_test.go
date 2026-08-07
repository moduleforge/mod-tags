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

func TestTagTemplateService_List_OneOfDomainSeededTrue(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	// Deliberately seed no tags at all — this is the direct proof of the
	// plan's core requirement, that the flag is readable via the catalog
	// before any real tag of the purpose exists.
	tagQ.seedPurposePolicy("priority", true)
	tagQ.seedTemplate("priority", "high", "High", nil, 0, 0, uuid.Nil)
	tagQ.seedTemplate("priority", "low", "Low", nil, 1, 0, uuid.Nil)

	got, err := svc.List(context.Background(), coreQ, tagQ, "priority", nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got): got %d, want 2", len(got))
	}
	for _, tt := range got {
		if !tt.OneOfDomain {
			t.Errorf("got[%q].OneOfDomain: got false, want true", tt.Value)
		}
	}
}

func TestTagTemplateService_List_OneOfDomainSeededFalse(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	// An explicit false row distinguishes itself from an absent row (see
	// TestTagTemplateService_List_OneOfDomainNoPolicyRow) at the service
	// boundary.
	tagQ.seedPurposePolicy("team", false)
	tagQ.seedTemplate("team", "eng", "Engineering", nil, 0, 0, uuid.Nil)

	got, err := svc.List(context.Background(), coreQ, tagQ, "team", nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got): got %d, want 1", len(got))
	}
	if got[0].OneOfDomain {
		t.Errorf("got[0].OneOfDomain: got true, want false")
	}
}

func TestTagTemplateService_List_OneOfDomainNoPolicyRow(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	// "unowned" is never passed to seedPurposePolicy, simulating a purpose
	// with no tag_purpose_policies row at all.
	tagQ.seedTemplate("unowned", "x", "X", nil, 0, 0, uuid.Nil)

	got, err := svc.List(context.Background(), coreQ, tagQ, "unowned", nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got): got %d, want 1", len(got))
	}
	if got[0].OneOfDomain {
		t.Errorf("got[0].OneOfDomain: got true, want false (no policy row)")
	}
}

// --- Upsert ---

func TestTagTemplateService_Upsert_Insert(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	appUUID, _ := coreQ.seedEntity("tag") // type slug irrelevant to Upsert
	color := "#00FF00FF"

	got, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope:     appUUID,
		Purpose:   "status",
		Value:     "active",
		Label:     "Active",
		Color:     &color,
		SortOrder: 3,
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.Purpose != "status" || got.Value != "active" || got.Label != "Active" || got.SortOrder != 3 {
		t.Errorf("got: %+v, want purpose=status value=active label=Active sortOrder=3", got)
	}
	if got.Color == nil || *got.Color != color {
		t.Errorf("got.Color: got %v, want %q", got.Color, color)
	}
	if got.Scope == nil || *got.Scope != appUUID {
		t.Errorf("got.Scope: got %v, want %v", got.Scope, appUUID)
	}
}

func TestTagTemplateService_Upsert_UpdatesOnConflict(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	appUUID, _ := coreQ.seedEntity("tag")
	firstColor := "#111111FF"
	secondColor := "#222222FF"

	first, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope:     appUUID,
		Purpose:   "status",
		Value:     "active",
		Label:     "Active",
		Color:     &firstColor,
		SortOrder: 0,
	})
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	second, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope:     appUUID,
		Purpose:   "status",
		Value:     "active",
		Label:     "Active (renamed)",
		Color:     &secondColor,
		SortOrder: 5,
	})
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	if len(tagQ.upsertedTemplates) != 1 {
		t.Fatalf("len(tagQ.upsertedTemplates): got %d, want 1 (update in place, not a duplicate row)",
			len(tagQ.upsertedTemplates))
	}
	if first.Label == second.Label {
		t.Errorf("expected label to change between calls; both are %q", first.Label)
	}
	if second.Label != "Active (renamed)" {
		t.Errorf("second.Label: got %q, want %q", second.Label, "Active (renamed)")
	}
	if second.Color == nil || *second.Color != secondColor {
		t.Errorf("second.Color: got %v, want %q", second.Color, secondColor)
	}
	if second.SortOrder != 5 {
		t.Errorf("second.SortOrder: got %d, want 5", second.SortOrder)
	}
}

func TestTagTemplateService_Upsert_DifferentValueDoesNotConflict(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	appUUID, _ := coreQ.seedEntity("tag")

	if _, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope: appUUID, Purpose: "status", Value: "active", Label: "Active",
	}); err != nil {
		t.Fatalf("Upsert active: %v", err)
	}
	if _, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope: appUUID, Purpose: "status", Value: "blocked", Label: "Blocked",
	}); err != nil {
		t.Fatalf("Upsert blocked: %v", err)
	}

	if len(tagQ.upsertedTemplates) != 2 {
		t.Fatalf("len(tagQ.upsertedTemplates): got %d, want 2 (distinct values, no conflict)",
			len(tagQ.upsertedTemplates))
	}
}

func TestTagTemplateService_Upsert_MissingFields(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}
	appUUID, _ := coreQ.seedEntity("tag")

	tests := []struct {
		name string
		in   UpsertTagTemplateInput
	}{
		{"missing purpose", UpsertTagTemplateInput{Scope: appUUID, Value: "v", Label: "L"}},
		{"missing value", UpsertTagTemplateInput{Scope: appUUID, Purpose: "p", Label: "L"}},
		{"missing label", UpsertTagTemplateInput{Scope: appUUID, Purpose: "p", Value: "v"}},
		{"missing scope", UpsertTagTemplateInput{Purpose: "p", Value: "v", Label: "L"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Upsert(context.Background(), coreQ, tagQ, tc.in)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err: got %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestTagTemplateService_Upsert_UnknownScope(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	svc := &TagTemplateService{}

	_, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope:   uuid.New(), // well-formed, never seeded
		Purpose: "status",
		Value:   "active",
		Label:   "Active",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err: got %v, want ErrInvalidInput (unknown scope is a genuine write error)", err)
	}
}

func TestTagTemplateService_Upsert_QueryError(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()
	tagQ.upsertTemplateErr = errors.New("boom")
	svc := &TagTemplateService{}

	appUUID, _ := coreQ.seedEntity("tag")
	_, err := svc.Upsert(context.Background(), coreQ, tagQ, UpsertTagTemplateInput{
		Scope: appUUID, Purpose: "status", Value: "active", Label: "Active",
	})
	if err == nil {
		t.Fatal("Upsert: got nil error, want non-nil")
	}
}

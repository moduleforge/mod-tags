package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/display"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// mockDisplayCoreQuerier is a minimal coredb.Querier for display registry tests.
type mockDisplayCoreQuerier struct {
	*mockCoreQuerier
}

func TestRegisterBuiltins_TagFieldName(t *testing.T) {
	coreQ := newMockCoreQuerier()
	tagQ := newMockTagQuerier()

	// Seed a tag entity in core.
	entityUUID, entityID := coreQ.seedEntity("tag")

	// Seed the tag row in tagQ.
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	tag := tagsdb.Tag{
		EntityID:  entityID,
		OwnerID:   1,
		SubjectID: 2,
		Purpose:   "status",
		Value:     "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	tagQ.tags[entityID] = tag

	// Seed the entity UUID mapping in coreQ so GetEntityByID works.
	entityRow := coreQ.entitiesByID[entityID]
	entityRow.Uuid = entityUUID
	entityRow.FundamentalTypeSlug = "tag"
	coreQ.entities[entityUUID] = entityRow
	coreQ.entitiesByID[entityID] = entityRow

	// Create the display registry.
	reg := display.NewRegistry(coreQ)
	RegisterBuiltins(reg, tagQ)

	// Seed a random UUID so GetEntityByUUID returns a "tag" slug.
	fakeEntityUUID := entityUUID

	// The Render call resolves entity slug via coreQ.GetEntityByID,
	// then calls the registered renderer for "tag" / "name".
	// We need to seed a known UUID → entity mapping in coreQ for Render.
	_ = fakeEntityUUID

	got, err := reg.Render(context.Background(), nil, entityID, display.FieldName)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	want := "status:active"
	if got != want {
		t.Errorf("Render: got %q, want %q", got, want)
	}
}

// mockDisplayTagQuerier is tagsdb.Querier that only implements GetTagByEntityID.
// Used so the display renderer test doesn't need a full mock.
type singleTagQuerier struct {
	tag   tagsdb.Tag
	errID int64
}

func (s *singleTagQuerier) CountTagsBySubjectEntityID(_ context.Context, _ tagsdb.CountTagsBySubjectEntityIDParams) (int64, error) {
	return 0, nil
}
func (s *singleTagQuerier) CreateTag(_ context.Context, _ tagsdb.CreateTagParams) (tagsdb.CreateTagRow, error) {
	return tagsdb.CreateTagRow{}, nil
}
func (s *singleTagQuerier) DeleteTag(_ context.Context, _ int64) error { return nil }
func (s *singleTagQuerier) GetTagByEntityID(_ context.Context, id int64) (tagsdb.Tag, error) {
	if id == s.errID {
		return tagsdb.Tag{}, nil
	}
	return s.tag, nil
}
func (s *singleTagQuerier) GetTagByEntityUUID(_ context.Context, _ uuid.UUID) (tagsdb.GetTagByEntityUUIDRow, error) {
	return tagsdb.GetTagByEntityUUIDRow{}, nil
}
func (s *singleTagQuerier) GetTagPurposePolicy(_ context.Context, _ string) (tagsdb.TagPurposePolicy, error) {
	return tagsdb.TagPurposePolicy{}, nil
}
func (s *singleTagQuerier) ListTagsBySubjectEntityID(_ context.Context, _ tagsdb.ListTagsBySubjectEntityIDParams) ([]tagsdb.ListTagsBySubjectEntityIDRow, error) {
	return nil, nil
}
func (s *singleTagQuerier) ListTagTemplates(_ context.Context, _ tagsdb.ListTagTemplatesParams) ([]tagsdb.ListTagTemplatesRow, error) {
	return nil, nil
}
func (s *singleTagQuerier) SearchTags(_ context.Context, _ tagsdb.SearchTagsParams) ([]tagsdb.SearchTagsRow, error) {
	return nil, nil
}
func (s *singleTagQuerier) UpdateTagColor(_ context.Context, _ tagsdb.UpdateTagColorParams) (tagsdb.UpdateTagColorRow, error) {
	return tagsdb.UpdateTagColorRow{}, nil
}
func (s *singleTagQuerier) UpdateTagValue(_ context.Context, _ tagsdb.UpdateTagValueParams) (tagsdb.UpdateTagValueRow, error) {
	return tagsdb.UpdateTagValueRow{}, nil
}
func (s *singleTagQuerier) UpsertTagPurposePolicy(_ context.Context, _ tagsdb.UpsertTagPurposePolicyParams) (tagsdb.TagPurposePolicy, error) {
	return tagsdb.TagPurposePolicy{}, nil
}
func (s *singleTagQuerier) UpsertTagTemplate(_ context.Context, _ tagsdb.UpsertTagTemplateParams) (tagsdb.UpsertTagTemplateRow, error) {
	return tagsdb.UpsertTagTemplateRow{}, nil
}

var _ tagsdb.Querier = (*singleTagQuerier)(nil)

func TestRegisterBuiltins_RenderReturnsExpectedFormat(t *testing.T) {
	coreQ := newMockCoreQuerier()

	entityID := int64(42)
	entityUUID := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}

	row := coredb.GetEntityByUUIDRow{
		ID:                  entityID,
		Uuid:                entityUUID,
		FundamentalTypeID:   coreQ.types["tag"].ID,
		FundamentalTypeSlug: "tag",
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	coreQ.entities[entityUUID] = row
	coreQ.entitiesByID[entityID] = row

	tagQ := &singleTagQuerier{
		tag: tagsdb.Tag{
			EntityID: entityID,
			Purpose:  "priority",
			Value:    "high",
		},
	}

	reg := display.NewRegistry(coreQ)
	RegisterBuiltins(reg, tagQ)

	got, err := reg.Render(context.Background(), nil, entityID, display.FieldName)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "priority:high"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

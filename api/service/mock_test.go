package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/txhelper"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// --- stub authz.Authorizer ---

// allowAllAuthz is a stub Authorizer that always permits every operation.
type allowAllAuthz struct{}

func (allowAllAuthz) Authorize(_ context.Context, _ string, _ *int64) error {
	return nil
}

// denyAllAuthz is a stub Authorizer that always rejects every operation.
type denyAllAuthz struct{ err error }

func (d denyAllAuthz) Authorize(_ context.Context, _ string, _ *int64) error {
	return d.err
}

// --- stub authz.OpResolver ---

// stubOpResolver is a stub OpResolver that returns a fixed set of op IDs.
// Tests that exercise list queries use this to satisfy the SatisfiedBy call.
type stubOpResolver struct {
	ids []int32
	err error
}

func newStubOpResolver() *stubOpResolver {
	// Return a representative set of op IDs (matches the seeded operations table).
	return &stubOpResolver{ids: []int32{1, 2, 3, 4, 5, 6, 7}}
}

func (r *stubOpResolver) SatisfiedBy(_ string) ([]int32, error) {
	return r.ids, r.err
}

// --- fake DB (txhelper.DB) and Tx ---

// fakeDB implements txhelper.DB. It returns the configured tx on BeginTx.
type fakeDB struct {
	tx  pgx.Tx
	err error
}

func (d *fakeDB) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return d.tx, d.err
}

var _ txhelper.DB = (*fakeDB)(nil)

// fakeTx is a minimal pgx.Tx that satisfies the interface for service tests.
// It records whether Commit and Rollback were called.
type fakeTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (t *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }
func (t *fakeTx) Commit(_ context.Context) error {
	t.committed = true
	return t.commitErr
}
func (t *fakeTx) Rollback(_ context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}
func (t *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults { return nil }
func (t *fakeTx) LargeObjects() pgx.LargeObjects                             { return pgx.LargeObjects{} }
func (t *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row        { return nil }
func (t *fakeTx) Conn() *pgx.Conn                                               { return nil }

var _ pgx.Tx = (*fakeTx)(nil)

// newFakeDB returns a fakeDB that commits/rolls back successfully.
func newFakeDB() *fakeDB {
	return &fakeDB{tx: &fakeTx{}}
}

// --- stub observer (recording) ---

// observeCall records a single Observe or ObserveAfterCommit invocation.
type observeCall struct {
	op             string
	resource       string
	targetEntityID *int64
	before         any
	after          any
}

// recordingObserver records Observe and ObserveAfterCommit calls.
type recordingObserver struct {
	observeCalls            []observeCall
	observeAfterCommitCalls []observeCall
	observeErr              error
}

func (o *recordingObserver) Observe(_ context.Context, _ pgx.Tx, op, resource string, targetEntityID *int64, before, after any) error {
	o.observeCalls = append(o.observeCalls, observeCall{
		op:             op,
		resource:       resource,
		targetEntityID: targetEntityID,
		before:         before,
		after:          after,
	})
	return o.observeErr
}

func (o *recordingObserver) ObserveAfterCommit(_ context.Context, op, resource string, targetEntityID *int64, after any) {
	o.observeAfterCommitCalls = append(o.observeAfterCommitCalls, observeCall{
		op:             op,
		resource:       resource,
		targetEntityID: targetEntityID,
		after:          after,
	})
}

// --- mock coredb.Querier ---

type mockCoreQuerier struct {
	entities                   map[uuid.UUID]coredb.GetEntityByUUIDRow
	entitiesByID               map[int64]coredb.GetEntityByUUIDRow
	types                      map[string]coredb.Type
	nextID                     int64
	archiveErr                 error
	createEntityWithOwnerCalls []coredb.CreateEntityWithOwnerParams // records CreateEntityWithOwner calls for assertion
}

func newMockCoreQuerier() *mockCoreQuerier {
	m := &mockCoreQuerier{
		entities:     make(map[uuid.UUID]coredb.GetEntityByUUIDRow),
		entitiesByID: make(map[int64]coredb.GetEntityByUUIDRow),
		types:        make(map[string]coredb.Type),
	}
	m.seedTypes()
	return m
}

func (m *mockCoreQuerier) seedTypes() {
	rows := []struct {
		id   int64
		slug string
	}{
		{1, "entity"},
		{2, "legal_entity"},
		{3, "natural_person"},
		{4, "corporation"},
		{5, "service_account"},
		{6, "tag"},
	}
	for _, r := range rows {
		m.types[r.slug] = coredb.Type{ID: r.id, Slug: r.slug, Name: r.slug, Concrete: true}
	}
}

func (m *mockCoreQuerier) nextSeq() int64 {
	m.nextID++
	return m.nextID
}

func (m *mockCoreQuerier) seedEntity(typeSlug string) (uuid.UUID, int64) {
	id := m.nextSeq()
	u := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	t := m.types[typeSlug]
	row := coredb.GetEntityByUUIDRow{
		ID:                  id,
		Uuid:                u,
		FundamentalTypeID:   t.ID,
		FundamentalTypeSlug: typeSlug,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.entities[u] = row
	m.entitiesByID[id] = row
	return u, id
}

func (m *mockCoreQuerier) ArchiveEntity(_ context.Context, argUuid uuid.UUID) error {
	if m.archiveErr != nil {
		return m.archiveErr
	}
	_, ok := m.entities[argUuid]
	if !ok {
		return pgx.ErrNoRows
	}
	delete(m.entities, argUuid)
	return nil
}

func (m *mockCoreQuerier) CreateEntity(_ context.Context, fundamentalTypeID int64) (coredb.Entity, error) {
	id := m.nextSeq()
	u := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	slug := ""
	for s, t := range m.types {
		if t.ID == fundamentalTypeID {
			slug = s
			break
		}
	}
	row := coredb.GetEntityByUUIDRow{
		ID:                  id,
		Uuid:                u,
		FundamentalTypeID:   fundamentalTypeID,
		FundamentalTypeSlug: slug,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.entities[u] = row
	m.entitiesByID[id] = row
	return coredb.Entity{
		ID:                id,
		Uuid:              u,
		FundamentalTypeID: fundamentalTypeID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// CreateEntityWithOwner sets ownership at INSERT time, mirroring the real
// query's avoidance of the entities_owner_immutable trigger (which only
// guards UPDATE). Records the call so tests can assert the owner was set
// correctly at entity-creation time.
func (m *mockCoreQuerier) CreateEntityWithOwner(_ context.Context, arg coredb.CreateEntityWithOwnerParams) (coredb.Entity, error) {
	m.createEntityWithOwnerCalls = append(m.createEntityWithOwnerCalls, arg)
	id := m.nextSeq()
	u := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	slug := ""
	for s, t := range m.types {
		if t.ID == arg.FundamentalTypeID {
			slug = s
			break
		}
	}
	row := coredb.GetEntityByUUIDRow{
		ID:                  id,
		Uuid:                u,
		FundamentalTypeID:   arg.FundamentalTypeID,
		FundamentalTypeSlug: slug,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.entities[u] = row
	m.entitiesByID[id] = row
	return coredb.Entity{
		ID:                id,
		Uuid:              u,
		FundamentalTypeID: arg.FundamentalTypeID,
		CreatedAt:         now,
		UpdatedAt:         now,
		OwnerID:           arg.OwnerID,
	}, nil
}

func (m *mockCoreQuerier) CreateCorporation(_ context.Context, _ coredb.CreateCorporationParams) (coredb.CreateCorporationRow, error) {
	return coredb.CreateCorporationRow{}, nil
}

func (m *mockCoreQuerier) CreateLegalEntity(_ context.Context, _ int64) (int64, error) { return 0, nil }

func (m *mockCoreQuerier) CreateNaturalPerson(_ context.Context, _ coredb.CreateNaturalPersonParams) (coredb.CreateNaturalPersonRow, error) {
	return coredb.CreateNaturalPersonRow{}, nil
}

func (m *mockCoreQuerier) ListAllTypes(_ context.Context) ([]coredb.Type, error) {
	var result []coredb.Type
	for _, t := range m.types {
		result = append(result, t)
	}
	return result, nil
}

func (m *mockCoreQuerier) CreateServiceAccount(_ context.Context, _ coredb.CreateServiceAccountParams) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}

func (m *mockCoreQuerier) GetCorporationByEntityID(_ context.Context, _ int64) (coredb.GetCorporationByEntityIDRow, error) {
	return coredb.GetCorporationByEntityIDRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetEntityByUUID(_ context.Context, argUuid uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	if e, ok := m.entities[argUuid]; ok {
		return e, nil
	}
	return coredb.GetEntityByUUIDRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetEntityByID(_ context.Context, id int64) (coredb.GetEntityByIDRow, error) {
	if e, ok := m.entitiesByID[id]; ok {
		return coredb.GetEntityByIDRow{
			ID:                  e.ID,
			Uuid:                e.Uuid,
			FundamentalTypeID:   e.FundamentalTypeID,
			FundamentalTypeSlug: e.FundamentalTypeSlug,
			CreatedAt:           e.CreatedAt,
			UpdatedAt:           e.UpdatedAt,
			ArchivedAt:          e.ArchivedAt,
		}, nil
	}
	return coredb.GetEntityByIDRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetLegalEntityByEntityID(_ context.Context, _ int64) (int64, error) {
	return 0, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetNaturalPersonByEntityID(_ context.Context, _ int64) (coredb.GetNaturalPersonByEntityIDRow, error) {
	return coredb.GetNaturalPersonByEntityIDRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetServiceAccountByEntityID(_ context.Context, _ int64) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetTypeBySlug(_ context.Context, slug string) (coredb.Type, error) {
	if t, ok := m.types[slug]; ok {
		return t, nil
	}
	return coredb.Type{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetTypeByID(_ context.Context, id int64) (coredb.Type, error) {
	for _, t := range m.types {
		if t.ID == id {
			return t, nil
		}
	}
	return coredb.Type{}, pgx.ErrNoRows
}

// --- apps-table stubs ---
// mod-core's coredb.Querier interface grew these apps-CRUD methods
// independently of tag-templates work; tag-template scope resolution uses
// only GetEntityByUUID (above). These are no-op stubs, added solely so
// mockCoreQuerier keeps satisfying the full interface — matching the
// existing not-implemented pattern already used for CreateLegalEntity et al.
func (m *mockCoreQuerier) GetAppBySlug(_ context.Context, _ string) (coredb.GetAppBySlugRow, error) {
	return coredb.GetAppBySlugRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) GetAppByUUID(_ context.Context, _ uuid.UUID) (coredb.GetAppByUUIDRow, error) {
	return coredb.GetAppByUUIDRow{}, pgx.ErrNoRows
}

func (m *mockCoreQuerier) InsertApp(_ context.Context, _ coredb.InsertAppParams) (coredb.InsertAppRow, error) {
	return coredb.InsertAppRow{}, nil
}

func (m *mockCoreQuerier) ListApps(_ context.Context) ([]coredb.ListAppsRow, error) {
	return nil, nil
}

func (m *mockCoreQuerier) UpdateApp(_ context.Context, _ coredb.UpdateAppParams) error {
	return nil
}

func (m *mockCoreQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error { return nil }

func (m *mockCoreQuerier) UpdateCorporation(_ context.Context, _ coredb.UpdateCorporationParams) error {
	return nil
}

func (m *mockCoreQuerier) UpdateNaturalPerson(_ context.Context, _ coredb.UpdateNaturalPersonParams) error {
	return nil
}

var _ coredb.Querier = (*mockCoreQuerier)(nil)

// --- mock tagsdb.Querier ---

type mockTagQuerier struct {
	tags             map[int64]tagsdb.Tag     // by entity_id
	tagsByUUID       map[uuid.UUID]tagsdb.Tag // by entity uuid
	entityUUID       map[int64]uuid.UUID      // entity_id → uuid
	uuidEntity       map[uuid.UUID]int64      // uuid → entity_id
	adminEntityIDs   map[int64]bool           // entity IDs treated as admin by access-fn simulation
	templates        []tagsdb.ListTagTemplatesRow
	nextID           int64
	createErr        error
	deleteErr        error
	updateErr        error
	listTemplatesErr error
}

func newMockTagQuerier() *mockTagQuerier {
	return &mockTagQuerier{
		tags:           make(map[int64]tagsdb.Tag),
		tagsByUUID:     make(map[uuid.UUID]tagsdb.Tag),
		entityUUID:     make(map[int64]uuid.UUID),
		uuidEntity:     make(map[uuid.UUID]int64),
		adminEntityIDs: make(map[int64]bool),
	}
}

// grantAdmin marks the given entity ID as an admin in the access-fn simulation.
func (m *mockTagQuerier) grantAdmin(entityID int64) {
	m.adminEntityIDs[entityID] = true
}

func (m *mockTagQuerier) nextSeq() int64 {
	m.nextID++
	return m.nextID
}

// seedTag inserts a tag with the given internal IDs and a fresh entity UUID.
func (m *mockTagQuerier) seedTag(entityID, ownerID, subjectID int64, purpose, value string, color *string) (uuid.UUID, tagsdb.Tag) {
	u := uuid.New()
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	colorParam := pgtype.Text{}
	if color != nil {
		colorParam = pgtype.Text{String: *color, Valid: true}
	}
	t := tagsdb.Tag{
		EntityID:  entityID,
		OwnerID:   ownerID,
		SubjectID: subjectID,
		Purpose:   purpose,
		Value:     value,
		Color:     colorParam,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.tags[entityID] = t
	m.tagsByUUID[u] = t
	m.entityUUID[entityID] = u
	m.uuidEntity[u] = entityID
	return u, t
}

func (m *mockTagQuerier) CountTagsBySubjectEntityID(_ context.Context, arg tagsdb.CountTagsBySubjectEntityIDParams) (int64, error) {
	var count int64
	for _, t := range m.tags {
		if t.SubjectID != arg.SubjectID {
			continue
		}
		// Simulate access function: actor is owner, subject, or admin (no admin
		// flag in params; treat any match on owner or subject as accessible).
		if arg.ActorEntityID == t.OwnerID || arg.ActorEntityID == t.SubjectID {
			count++
		}
	}
	return count, nil
}

func (m *mockTagQuerier) CreateTag(_ context.Context, arg tagsdb.CreateTagParams) (tagsdb.Tag, error) {
	if m.createErr != nil {
		return tagsdb.Tag{}, m.createErr
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	t := tagsdb.Tag{
		EntityID:  arg.EntityID,
		OwnerID:   arg.OwnerID,
		SubjectID: arg.SubjectID,
		Purpose:   arg.Purpose,
		Value:     arg.Value,
		Color:     arg.Color,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.tags[arg.EntityID] = t
	return t, nil
}

func (m *mockTagQuerier) DeleteTag(_ context.Context, entityID int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.tags, entityID)
	// Also remove from UUID maps.
	if u, ok := m.entityUUID[entityID]; ok {
		delete(m.tagsByUUID, u)
		delete(m.uuidEntity, u)
		delete(m.entityUUID, entityID)
	}
	return nil
}

func (m *mockTagQuerier) GetTagByEntityID(_ context.Context, entityID int64) (tagsdb.Tag, error) {
	if t, ok := m.tags[entityID]; ok {
		return t, nil
	}
	return tagsdb.Tag{}, pgx.ErrNoRows
}

func (m *mockTagQuerier) GetTagByEntityUUID(_ context.Context, argUuid uuid.UUID) (tagsdb.GetTagByEntityUUIDRow, error) {
	if t, ok := m.tagsByUUID[argUuid]; ok {
		return tagsdb.GetTagByEntityUUIDRow{
			EntityID:  t.EntityID,
			OwnerID:   t.OwnerID,
			SubjectID: t.SubjectID,
			Purpose:   t.Purpose,
			Value:     t.Value,
			Color:     t.Color,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
			Uuid:      argUuid,
		}, nil
	}
	return tagsdb.GetTagByEntityUUIDRow{}, pgx.ErrNoRows
}

func (m *mockTagQuerier) ListTagsBySubjectEntityID(_ context.Context, arg tagsdb.ListTagsBySubjectEntityIDParams) ([]tagsdb.ListTagsBySubjectEntityIDRow, error) {
	var result []tagsdb.ListTagsBySubjectEntityIDRow
	for _, t := range m.tags {
		if t.SubjectID != arg.SubjectID {
			continue
		}
		if arg.Purpose.Valid && t.Purpose != arg.Purpose.String {
			continue
		}
		// Simulate access function: actor is admin, owner, or subject.
		if !m.adminEntityIDs[arg.ActorEntityID] && arg.ActorEntityID != t.OwnerID && arg.ActorEntityID != t.SubjectID {
			continue
		}
		u, ok := m.entityUUID[t.EntityID]
		if !ok {
			u = uuid.Nil
		}
		result = append(result, tagsdb.ListTagsBySubjectEntityIDRow{
			EntityID:  t.EntityID,
			OwnerID:   t.OwnerID,
			SubjectID: t.SubjectID,
			Purpose:   t.Purpose,
			Value:     t.Value,
			Color:     t.Color,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
			Uuid:      u,
		})
	}
	return result, nil
}

func (m *mockTagQuerier) SearchTags(_ context.Context, arg tagsdb.SearchTagsParams) ([]tagsdb.SearchTagsRow, error) {
	var result []tagsdb.SearchTagsRow
	for _, t := range m.tags {
		if arg.OwnerID.Valid && t.OwnerID != arg.OwnerID.Int64 {
			continue
		}
		if arg.SubjectID.Valid && t.SubjectID != arg.SubjectID.Int64 {
			continue
		}
		if arg.Purpose.Valid && t.Purpose != arg.Purpose.String {
			continue
		}
		if arg.Value.Valid && t.Value != arg.Value.String {
			continue
		}
		// Simulate access function: actor is admin, owner, or subject.
		if !m.adminEntityIDs[arg.ActorEntityID] && arg.ActorEntityID != t.OwnerID && arg.ActorEntityID != t.SubjectID {
			continue
		}
		u, ok := m.entityUUID[t.EntityID]
		if !ok {
			u = uuid.Nil
		}
		result = append(result, tagsdb.SearchTagsRow{
			EntityID:  t.EntityID,
			OwnerID:   t.OwnerID,
			SubjectID: t.SubjectID,
			Purpose:   t.Purpose,
			Value:     t.Value,
			Color:     t.Color,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
			Uuid:      u,
		})
	}
	return result, nil
}

func (m *mockTagQuerier) UpdateTagColor(_ context.Context, arg tagsdb.UpdateTagColorParams) (tagsdb.Tag, error) {
	if m.updateErr != nil {
		return tagsdb.Tag{}, m.updateErr
	}
	t, ok := m.tags[arg.EntityID]
	if !ok {
		return tagsdb.Tag{}, pgx.ErrNoRows
	}
	t.Color = arg.Color
	t.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.tags[arg.EntityID] = t
	// update tagsByUUID too
	if u, ok2 := m.entityUUID[arg.EntityID]; ok2 {
		m.tagsByUUID[u] = t
	}
	return t, nil
}

func (m *mockTagQuerier) UpdateTagValue(_ context.Context, arg tagsdb.UpdateTagValueParams) (tagsdb.Tag, error) {
	if m.updateErr != nil {
		return tagsdb.Tag{}, m.updateErr
	}
	t, ok := m.tags[arg.EntityID]
	if !ok {
		return tagsdb.Tag{}, pgx.ErrNoRows
	}
	t.Value = arg.Value
	t.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.tags[arg.EntityID] = t
	// update tagsByUUID too
	if u, ok2 := m.entityUUID[arg.EntityID]; ok2 {
		m.tagsByUUID[u] = t
	}
	return t, nil
}

// seedTemplate inserts a tag_templates row. Pass a zero scopeID (0) for a
// global template; a non-zero scopeID/scopeUUID pair for a scoped template.
func (m *mockTagQuerier) seedTemplate(purpose, value, label string, color *string, sortOrder int32, scopeID int64, scopeUUID uuid.UUID) {
	colorParam := pgtype.Text{}
	if color != nil {
		colorParam = pgtype.Text{String: *color, Valid: true}
	}
	var scopeParam pgtype.Int8
	var scopeUUIDParam pgtype.UUID
	if scopeID != 0 {
		scopeParam = pgtype.Int8{Int64: scopeID, Valid: true}
		scopeUUIDParam = pgtype.UUID{Bytes: scopeUUID, Valid: true}
	}
	m.templates = append(m.templates, tagsdb.ListTagTemplatesRow{
		Purpose:   purpose,
		Value:     value,
		Label:     label,
		Color:     colorParam,
		SortOrder: sortOrder,
		Scope:     scopeParam,
		ScopeUuid: scopeUUIDParam,
	})
}

// ListTagTemplates simulates the generated query: rows matching purpose,
// where a global row (Scope invalid) always matches and a scoped row
// matches only when arg.Scope is valid and equal.
func (m *mockTagQuerier) ListTagTemplates(_ context.Context, arg tagsdb.ListTagTemplatesParams) ([]tagsdb.ListTagTemplatesRow, error) {
	if m.listTemplatesErr != nil {
		return nil, m.listTemplatesErr
	}
	var result []tagsdb.ListTagTemplatesRow
	for _, t := range m.templates {
		if t.Purpose != arg.Purpose {
			continue
		}
		if t.Scope.Valid && (!arg.Scope.Valid || t.Scope.Int64 != arg.Scope.Int64) {
			continue
		}
		result = append(result, t)
	}
	return result, nil
}

var _ tagsdb.Querier = (*mockTagQuerier)(nil)

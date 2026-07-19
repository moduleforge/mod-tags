package httpapi

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"

	coredb "github.com/moduleforge/core-model/db"
	"github.com/moduleforge/tags-api/service"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// --- fake TagServicer ---

type fakeTagService struct {
	tag  service.Tag
	tags []service.Tag
	err  error
}

func (f *fakeTagService) Create(_ context.Context, _ coredb.Querier, _ service.CreateTagInput) (service.Tag, error) {
	return f.tag, f.err
}

func (f *fakeTagService) GetByUUID(_ context.Context, _ coredb.Querier, _ tagsdb.Querier, _ uuid.UUID) (service.Tag, error) {
	return f.tag, f.err
}

func (f *fakeTagService) Search(_ context.Context, _ coredb.Querier, _ tagsdb.Querier, _ service.SearchTagsFilter, _ service.Pagination) ([]service.Tag, error) {
	return f.tags, f.err
}

func (f *fakeTagService) ListBySubject(_ context.Context, _ coredb.Querier, _ tagsdb.Querier, _ uuid.UUID, _ *string, _ service.Pagination) ([]service.Tag, error) {
	return f.tags, f.err
}

func (f *fakeTagService) UpdateByUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateTagInput) (service.Tag, error) {
	return f.tag, f.err
}

func (f *fakeTagService) UpdateTagValue(_ context.Context, _ coredb.Querier, _ uuid.UUID, _ service.UpdateTagValueInput) (service.Tag, error) {
	return f.tag, f.err
}

func (f *fakeTagService) DeleteByUUID(_ context.Context, _ coredb.Querier, _ uuid.UUID) error {
	return f.err
}

var _ service.TagServicer = (*fakeTagService)(nil)

// --- fake TagTemplateServicer ---

type fakeTagTemplateService struct {
	templates []service.TagTemplate
	err       error
}

func (f *fakeTagTemplateService) List(_ context.Context, _ coredb.Querier, _ tagsdb.Querier, _ string, _ *uuid.UUID) ([]service.TagTemplate, error) {
	return f.templates, f.err
}

var _ service.TagTemplateServicer = (*fakeTagTemplateService)(nil)

// --- fake coredb.Querier (pass-through; real work done by fakeTagService) ---

type fakeCoreQuerier struct{}

func (f *fakeCoreQuerier) ArchiveEntity(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeCoreQuerier) CreateCorporation(_ context.Context, _ coredb.CreateCorporationParams) (coredb.CreateCorporationRow, error) {
	return coredb.CreateCorporationRow{}, nil
}
func (f *fakeCoreQuerier) CreateEntity(_ context.Context, _ int64) (coredb.Entity, error) {
	return coredb.Entity{}, nil
}
func (f *fakeCoreQuerier) CreateEntityWithOwner(_ context.Context, _ coredb.CreateEntityWithOwnerParams) (coredb.Entity, error) {
	return coredb.Entity{}, nil
}
func (f *fakeCoreQuerier) CreateLegalEntity(_ context.Context, _ int64) (int64, error) { return 0, nil }
func (f *fakeCoreQuerier) CreateNaturalPerson(_ context.Context, _ coredb.CreateNaturalPersonParams) (coredb.CreateNaturalPersonRow, error) {
	return coredb.CreateNaturalPersonRow{}, nil
}

func (f *fakeCoreQuerier) ListAllTypes(_ context.Context) ([]coredb.Type, error) {
	return nil, nil
}
func (f *fakeCoreQuerier) CreateServiceAccount(_ context.Context, _ coredb.CreateServiceAccountParams) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (f *fakeCoreQuerier) GetCorporationByEntityID(_ context.Context, _ int64) (coredb.GetCorporationByEntityIDRow, error) {
	return coredb.GetCorporationByEntityIDRow{}, nil
}
func (f *fakeCoreQuerier) GetEntityByID(_ context.Context, _ int64) (coredb.GetEntityByIDRow, error) {
	return coredb.GetEntityByIDRow{}, nil
}
func (f *fakeCoreQuerier) GetEntityByUUID(_ context.Context, _ uuid.UUID) (coredb.GetEntityByUUIDRow, error) {
	return coredb.GetEntityByUUIDRow{}, nil
}
func (f *fakeCoreQuerier) GetLegalEntityByEntityID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (f *fakeCoreQuerier) GetNaturalPersonByEntityID(_ context.Context, _ int64) (coredb.GetNaturalPersonByEntityIDRow, error) {
	return coredb.GetNaturalPersonByEntityIDRow{}, nil
}
func (f *fakeCoreQuerier) GetServiceAccountByEntityID(_ context.Context, _ int64) (coredb.ServiceAccount, error) {
	return coredb.ServiceAccount{}, nil
}
func (f *fakeCoreQuerier) GetTypeBySlug(_ context.Context, _ string) (coredb.Type, error) {
	return coredb.Type{}, nil
}
func (f *fakeCoreQuerier) GetTypeByID(_ context.Context, _ int64) (coredb.Type, error) {
	return coredb.Type{}, nil
}

// --- apps-table stubs ---
// mod-core's coredb.Querier interface grew these apps-CRUD methods
// independently of tag-templates work; the tag-templates handler never
// calls them (scope resolution happens in the service layer, not here).
// No-op stubs, added solely so fakeCoreQuerier keeps satisfying the full
// interface — matching the existing not-implemented pattern already used
// for CreateLegalEntity et al.
func (f *fakeCoreQuerier) GetAppBySlug(_ context.Context, _ string) (coredb.GetAppBySlugRow, error) {
	return coredb.GetAppBySlugRow{}, nil
}
func (f *fakeCoreQuerier) GetAppByUUID(_ context.Context, _ uuid.UUID) (coredb.GetAppByUUIDRow, error) {
	return coredb.GetAppByUUIDRow{}, nil
}
func (f *fakeCoreQuerier) InsertApp(_ context.Context, _ coredb.InsertAppParams) (coredb.InsertAppRow, error) {
	return coredb.InsertAppRow{}, nil
}
func (f *fakeCoreQuerier) ListApps(_ context.Context) ([]coredb.ListAppsRow, error) {
	return nil, nil
}
func (f *fakeCoreQuerier) UpdateApp(_ context.Context, _ coredb.UpdateAppParams) error {
	return nil
}

func (f *fakeCoreQuerier) UnarchiveEntity(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeCoreQuerier) UpdateCorporation(_ context.Context, _ coredb.UpdateCorporationParams) error {
	return nil
}
func (f *fakeCoreQuerier) UpdateNaturalPerson(_ context.Context, _ coredb.UpdateNaturalPersonParams) error {
	return nil
}

var _ coredb.Querier = (*fakeCoreQuerier)(nil)

// noopLogger returns a slog.Logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// buildTestDeps builds a Deps with mocked services.
// The fakeTagService receives all handler calls; queriers are no-ops.
func buildTestDeps(tagSvc *fakeTagService) Deps {
	if tagSvc == nil {
		tagSvc = &fakeTagService{}
	}
	svcs := &service.Services{}
	svcs.Tag = tagSvc
	svcs.TagTemplate = &fakeTagTemplateService{}

	return Deps{
		CoreQuerier: &fakeCoreQuerier{},
		Services:    svcs,
		Logger:      noopLogger(),
	}
}

// buildTestDepsForTemplates builds a Deps with a mocked TagTemplateServicer.
// The tags service (fakeTagService) is a no-op default — tag-template
// handler tests never exercise it.
func buildTestDepsForTemplates(tmplSvc *fakeTagTemplateService) Deps {
	if tmplSvc == nil {
		tmplSvc = &fakeTagTemplateService{}
	}
	svcs := &service.Services{}
	svcs.Tag = &fakeTagService{}
	svcs.TagTemplate = tmplSvc

	return Deps{
		CoreQuerier: &fakeCoreQuerier{},
		Services:    svcs,
		Logger:      noopLogger(),
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/moduleforge/core-api/authz"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	coresvc "github.com/moduleforge/core-api/service"
	"github.com/moduleforge/core-api/txhelper"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// colorRe validates the color format "#RRGGBBAA" (8 hex digits after #).
var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{8}$`)

// pgUniqueViolation is the Postgres error code for unique_violation.
const pgUniqueViolation = "23505"

// CreateTagInput carries the fields required to create a tag.
type CreateTagInput struct {
	SubjectEntityUUID uuid.UUID
	Purpose           string
	Value             string
	Color             *string // optional; must match "#RRGGBBAA" if set
}

// UpdateTagInput carries the only mutable field on a tag.
type UpdateTagInput struct {
	// Color = nil clears the color (sets DB column to NULL).
	// Color = &"" is invalid (rejected by regex).
	// Color = &"#RRGGBBAA" sets the color.
	Color *string
}

// UpdateTagValueInput carries the new value for a tag update.
type UpdateTagValueInput struct {
	Value string
}

// SearchTagsFilter filters the tag search. At least one of OwnerEntityUUID /
// SubjectEntityUUID is required; the rest are optional.
type SearchTagsFilter struct {
	OwnerEntityUUID   *uuid.UUID
	SubjectEntityUUID *uuid.UUID
	Purpose           *string
	Value             *string
}

// Pagination is the shared limit/offset type from core. Re-exported here so
// callers in this package do not need to import core-api/service directly.
type Pagination = coresvc.Pagination

// Tag is the service-layer representation of a tag, using public UUIDs.
type Tag struct {
	EntityUUID  uuid.UUID
	OwnerUUID   uuid.UUID
	SubjectUUID uuid.UUID
	Purpose     string
	Value       string
	Color       *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TagServicer defines tag CRUD operations available to httpapi handlers.
type TagServicer interface {
	Create(ctx context.Context, coreQ coredb.Querier, in CreateTagInput) (Tag, error)
	GetByUUID(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, entityUUID uuid.UUID) (Tag, error)
	Search(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, filter SearchTagsFilter, p Pagination) ([]Tag, error)
	ListBySubject(ctx context.Context, coreQ coredb.Querier, tagQ tagsdb.Querier, subjectUUID uuid.UUID, purposeFilter *string, p Pagination) ([]Tag, error)
	UpdateByUUID(ctx context.Context, coreQ coredb.Querier, entityUUID uuid.UUID, in UpdateTagInput) (Tag, error)
	UpdateTagValue(ctx context.Context, coreQ coredb.Querier, entityUUID uuid.UUID, in UpdateTagValueInput) (Tag, error)
	DeleteByUUID(ctx context.Context, coreQ coredb.Querier, entityUUID uuid.UUID) error
}

// TagService implements TagServicer with authorization, transactional mutation,
// and observer dispatch.
type TagService struct {
	db             txhelper.DB
	az             authz.Authorizer
	opRes          authz.OpResolver
	obs            *observer.ObserverGroup
	resolver       *types.Resolver
	entityResolver *entity.Resolver
	newCoreQuerier func(pgx.Tx) coredb.Querier // injectable for tests; defaults to coredb.New
	newTagQuerier  func(pgx.Tx) tagsdb.Querier // injectable for tests; defaults to tagsdb.New
}

// Compile-time assertion.
var _ TagServicer = (*TagService)(nil)

func (s *TagService) coreQuerier(tx pgx.Tx) coredb.Querier {
	if s.newCoreQuerier != nil {
		return s.newCoreQuerier(tx)
	}
	return coredb.New(tx)
}

func (s *TagService) tagQuerier(tx pgx.Tx) tagsdb.Querier {
	if s.newTagQuerier != nil {
		return s.newTagQuerier(tx)
	}
	return tagsdb.New(tx)
}

// Create inserts an entity row and a tags row in a single transaction.
// Owner is set server-side to the actor's entity ID from opctx — any
// client-supplied owner is ignored. The subject entity must already exist.
// Returns ErrForbidden if no actor is present in ctx.
func (s *TagService) Create(
	ctx context.Context,
	coreQ coredb.Querier,
	in CreateTagInput,
) (Tag, error) {
	actorEntityID, ok := opctx.ActorEntityID(ctx)
	if !ok {
		return Tag{}, ErrForbidden
	}

	// 1. Authorize against the tag type ID (entity-level create convention).
	tagTypeID := s.resolver.IDForSlugMust("tag")
	if err := s.az.Authorize(ctx, "create", &tagTypeID); err != nil {
		return Tag{}, err
	}

	// Input validation.
	in.Purpose = strings.TrimSpace(in.Purpose)
	in.Value = strings.TrimSpace(in.Value)
	if in.Purpose == "" {
		return Tag{}, fmt.Errorf("%w: purpose is required", ErrInvalidInput)
	}
	if len(in.Purpose) > 512 {
		return Tag{}, fmt.Errorf("%w: purpose exceeds 512 characters", ErrInvalidInput)
	}
	if in.Value == "" {
		return Tag{}, fmt.Errorf("%w: value is required", ErrInvalidInput)
	}
	if len(in.Value) > 512 {
		return Tag{}, fmt.Errorf("%w: value exceeds 512 characters", ErrInvalidInput)
	}
	if in.Color != nil && !colorRe.MatchString(*in.Color) {
		return Tag{}, fmt.Errorf("%w: color must match #RRGGBBAA", ErrInvalidInput)
	}

	// Resolve subject entity (pre-tx read). Existence-masking: a genuine miss
	// surfaces as ErrForbidden (403), not ErrNotFound (404), via
	// entityResolver.Resolve. "subject_entity" is a distinct, un-opted-in
	// slug (not "tag") so a future AllowNotFound("tag") opt-in on the tag
	// resource cannot accidentally un-mask subject lookups.
	subjectEntityID, err := s.entityResolver.Resolve(ctx, coreQ, in.SubjectEntityUUID, "subject_entity")
	if err != nil {
		return Tag{}, err
	}

	// Resolve owner entity (actor) via pre-tx read.
	ownerEntity, err := coreQ.GetEntityByID(ctx, actorEntityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tag{}, fmt.Errorf("%w: owner entity not found", ErrNotFound)
		}
		return Tag{}, fmt.Errorf("tag.Create resolve owner: %w", err)
	}

	// 2. Mutate inside a transaction; observers participate in the same tx.
	var result Tag
	var entityID int64

	err = txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		txCoreQ := s.coreQuerier(tx)
		txTagQ := s.tagQuerier(tx)

		// Insert entity row with ownership set at INSERT time. entities.owner_id
		// is immutable after insert (entities_owner_immutable trigger fires on
		// any UPDATE OF owner_id, including the first NULL -> value write), so
		// ownership must be set here rather than via a follow-up UPDATE. The
		// same actor value is used for the local tags.owner_id write below so
		// the two columns can never diverge.
		entity, err := txCoreQ.CreateEntityWithOwner(ctx, coredb.CreateEntityWithOwnerParams{
			FundamentalTypeID: tagTypeID,
			OwnerID:           pgtype.Int8{Int64: actorEntityID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("tag.Create entity: %w", err)
		}
		entityID = entity.ID

		// Build color param.
		colorParam := pgtype.Text{}
		if in.Color != nil {
			colorParam = pgtype.Text{String: *in.Color, Valid: true}
		}

		// Insert tags row.
		tag, err := txTagQ.CreateTag(ctx, tagsdb.CreateTagParams{
			EntityID:  entity.ID,
			OwnerID:   actorEntityID,
			SubjectID: subjectEntityID,
			Purpose:   in.Purpose,
			Value:     in.Value,
			Color:     colorParam,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
				return fmt.Errorf("%w: tag already exists", ErrConflict)
			}
			return fmt.Errorf("tag.Create insert: %w", err)
		}

		result = hydrateTag(entity.Uuid, ownerEntity.Uuid, in.SubjectEntityUUID, tag)

		after := tagSnapshot(result)
		return s.obs.Observe(ctx, tx, "create", "tag", &entityID, nil, after)
	})
	if err != nil {
		return Tag{}, err
	}

	// 3. Post-commit observers (optional for tags — no search-index or cache consumer yet).
	s.obs.ObserveAfterCommit(ctx, "create", "tag", &entityID, tagSnapshot(result))

	return result, nil
}

// GetByUUID resolves a tag by entity UUID and enforces read authz.
// Access control (owner or subject) is enforced by the grants-driven Authorizer;
// the inline filter is no longer needed here.
func (s *TagService) GetByUUID(
	ctx context.Context,
	coreQ coredb.Querier,
	tagQ tagsdb.Querier,
	entityUUID uuid.UUID,
) (Tag, error) {
	// 1. Resolve UUID → internal entity ID. The default not-found policy
	// returns ErrForbidden (masking existence); apps can opt this resource
	// into 404 via EntityResolver.AllowNotFound.
	tagEntityID, err := s.entityResolver.Resolve(ctx, coreQ, entityUUID, "tag")
	if err != nil {
		return Tag{}, err
	}

	// 2. Authorize the read against the resolved entity ID.
	// The Authorizer enforces owner OR subject access via the per-resource
	// own predicate in the GrantTableGenerator — no inline filter needed.
	if err := s.az.Authorize(ctx, "read", &tagEntityID); err != nil {
		return Tag{}, err
	}

	row, err := tagQ.GetTagByEntityUUID(ctx, entityUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tag{}, ErrNotFound
		}
		return Tag{}, fmt.Errorf("tag.GetByUUID: %w", err)
	}

	// Resolve UUIDs for owner and subject.
	ownerEntity, err := coreQ.GetEntityByID(ctx, row.OwnerID)
	if err != nil {
		return Tag{}, fmt.Errorf("tag.GetByUUID resolve owner: %w", err)
	}
	subjectEntity, err := coreQ.GetEntityByID(ctx, row.SubjectID)
	if err != nil {
		return Tag{}, fmt.Errorf("tag.GetByUUID resolve subject: %w", err)
	}

	return hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, tagFromUUIDRow(row)), nil
}

// Search finds tags matching filter. Row-level scoping is enforced in SQL via
// the accessible_tag_ids_for_actor join; no post-query Go filtering needed.
func (s *TagService) Search(
	ctx context.Context,
	coreQ coredb.Querier,
	tagQ tagsdb.Querier,
	filter SearchTagsFilter,
	p Pagination,
) ([]Tag, error) {
	actorEntityID, ok := opctx.ActorEntityID(ctx)
	if !ok {
		return nil, ErrForbidden
	}

	// 1. Authorize against the tag type ID (entity-level list convention).
	tagTypeID := s.resolver.IDForSlugMust("tag")
	if err := s.az.Authorize(ctx, "list", &tagTypeID); err != nil {
		return nil, err
	}

	if filter.OwnerEntityUUID == nil && filter.SubjectEntityUUID == nil {
		return nil, fmt.Errorf("%w: at least one of owner or subject is required", ErrInvalidInput)
	}

	// op_ids covers the read closure (read, sread, update, delete, swrite, manage)
	// per the SatisfiedBy contract — anyone with any of these grants can see the row.
	opIDs, err := s.opRes.SatisfiedBy("read")
	if err != nil {
		return nil, fmt.Errorf("tag.Search resolve op_ids: %w", err)
	}

	limit, offset := p.Normalize()
	params := tagsdb.SearchTagsParams{
		ActorEntityID: actorEntityID,
		OpIds:         opIDs,
		Limit:         limit,
		Offset:        offset,
	}

	if filter.OwnerEntityUUID != nil {
		// Deliberate behaviour change (flagged in the task doc, distinct from
		// the 404->403 fixes above): a genuine miss on the owner filter used
		// to return an empty result set (already non-leaking). Per the
		// manager's "full coverage" instruction and the masking-by-default
		// policy, route it through entityResolver.Resolve so a genuine miss
		// now masks to ErrForbidden (403) instead. "owner_entity" is a
		// distinct, un-opted-in slug (not "tag").
		ownerEntityID, err := s.entityResolver.Resolve(ctx, coreQ, *filter.OwnerEntityUUID, "owner_entity")
		if err != nil {
			return nil, err
		}
		params.OwnerID = pgtype.Int8{Int64: ownerEntityID, Valid: true}
	}

	if filter.SubjectEntityUUID != nil {
		// Deliberate behaviour change (flagged in the task doc): same
		// treatment as the owner filter above. Same "subject_entity" slug as
		// Create/ListBySubject's subject resolutions.
		subjectEntityID, err := s.entityResolver.Resolve(ctx, coreQ, *filter.SubjectEntityUUID, "subject_entity")
		if err != nil {
			return nil, err
		}
		params.SubjectID = pgtype.Int8{Int64: subjectEntityID, Valid: true}
	}

	if filter.Purpose != nil {
		params.Purpose = pgtype.Text{String: *filter.Purpose, Valid: true}
	}
	if filter.Value != nil {
		params.Value = pgtype.Text{String: *filter.Value, Valid: true}
	}

	rows, err := tagQ.SearchTags(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("tag.Search query: %w", err)
	}

	// SQL access function enforces row-level scoping; no Go-side post-filter.
	var result []Tag
	for _, row := range rows {
		ownerEntity, err := coreQ.GetEntityByID(ctx, row.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("tag.Search resolve owner uuid: %w", err)
		}
		subjectEntity, err := coreQ.GetEntityByID(ctx, row.SubjectID)
		if err != nil {
			return nil, fmt.Errorf("tag.Search resolve subject uuid: %w", err)
		}
		result = append(result, hydrateTagFromSearchRow(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, row))
	}

	if result == nil {
		result = []Tag{}
	}
	return result, nil
}

// ListBySubject returns all tags targeting a given subject entity. Row-level
// scoping is enforced in SQL via the accessible_tag_ids_for_actor join; no
// post-query Go filtering needed.
func (s *TagService) ListBySubject(
	ctx context.Context,
	coreQ coredb.Querier,
	tagQ tagsdb.Querier,
	subjectUUID uuid.UUID,
	purposeFilter *string,
	p Pagination,
) ([]Tag, error) {
	actorEntityID, ok := opctx.ActorEntityID(ctx)
	if !ok {
		return nil, ErrForbidden
	}

	// 1. Resolve the subject entity first so we can authorize against its ID.
	// Existence-masking: this list route is scoped to the subject/parent
	// entity, so an inaccessible subject must mask exactly like a direct
	// lookup — a genuine miss surfaces as ErrForbidden (403), not
	// ErrNotFound (404), via entityResolver.Resolve. Same "subject_entity"
	// slug as Create's subject resolution (not "tag").
	subjectEntityID, err := s.entityResolver.Resolve(ctx, coreQ, subjectUUID, "subject_entity")
	if err != nil {
		return nil, err
	}

	// Authorize against the subject entity ID.
	if err := s.az.Authorize(ctx, "list", &subjectEntityID); err != nil {
		return nil, err
	}

	// op_ids covers the read closure (read, sread, update, delete, swrite, manage)
	// per the SatisfiedBy contract — anyone with any of these grants can see the row.
	opIDs, err := s.opRes.SatisfiedBy("read")
	if err != nil {
		return nil, fmt.Errorf("tag.ListBySubject resolve op_ids: %w", err)
	}

	purposeParam := pgtype.Text{}
	if purposeFilter != nil {
		purposeParam = pgtype.Text{String: *purposeFilter, Valid: true}
	}

	limit, offset := p.Normalize()
	rows, err := tagQ.ListTagsBySubjectEntityID(ctx, tagsdb.ListTagsBySubjectEntityIDParams{
		ActorEntityID: actorEntityID,
		OpIds:         opIDs,
		SubjectID:     subjectEntityID,
		Purpose:       purposeParam,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return nil, fmt.Errorf("tag.ListBySubject query: %w", err)
	}

	// SQL access function enforces row-level scoping; no Go-side post-filter.
	var result []Tag
	for _, row := range rows {
		ownerEntity, err := coreQ.GetEntityByID(ctx, row.OwnerID)
		if err != nil {
			return nil, fmt.Errorf("tag.ListBySubject resolve owner uuid: %w", err)
		}
		result = append(result, hydrateTagFromListRow(row.Uuid, ownerEntity.Uuid, subjectUUID, row))
	}

	if result == nil {
		result = []Tag{}
	}
	return result, nil
}

// UpdateByUUID updates the color of a tag identified by entity UUID.
// Row-level access control (owner only, not subject) is enforced by the
// Authorizer; the inline filter is no longer needed.
func (s *TagService) UpdateByUUID(
	ctx context.Context,
	coreQ coredb.Querier,
	entityUUID uuid.UUID,
	in UpdateTagInput,
) (Tag, error) {
	// Validate non-nil color before any DB call.
	if in.Color != nil && !colorRe.MatchString(*in.Color) {
		return Tag{}, fmt.Errorf("%w: color must match #RRGGBBAA", ErrInvalidInput)
	}

	// 1. Resolve the tag's entity UUID up front (pre-tx), mirroring
	// GetByUUID's pre-tx Resolve call: a genuine miss surfaces as
	// ErrForbidden (403) directly, before entering the tx.
	if _, err := s.entityResolver.Resolve(ctx, coreQ, entityUUID, "tag"); err != nil {
		return Tag{}, err
	}

	// 2. Authorize — we authorize before fetching; use a stub target
	//    (no entity ID yet since we haven't fetched).
	if err := s.az.Authorize(ctx, "update", nil); err != nil {
		return Tag{}, err
	}

	// 3. Mutate inside a transaction; observers participate in the same tx.
	var result Tag
	var entityID int64

	err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		txCoreQ := s.coreQuerier(tx)
		txTagQ := s.tagQuerier(tx)

		// Fetch the existing tag (before snapshot). entityResolver.Resolve
		// already confirmed the entity exists (pre-tx, above); a miss here
		// is a data-consistency case (entity resolved, tags row absent), so
		// it also masks to ErrForbidden, not ErrNotFound — masking-consistent
		// per the task's cross-cutting note, avoiding a residual gap.
		row, err := txTagQ.GetTagByEntityUUID(ctx, entityUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrForbidden
			}
			return fmt.Errorf("tag.UpdateByUUID fetch: %w", err)
		}

		entityID = row.EntityID
		before := colorSnapshot(row.Color)

		colorParam := pgtype.Text{}
		if in.Color != nil {
			colorParam = pgtype.Text{String: *in.Color, Valid: true}
		}

		updated, err := txTagQ.UpdateTagColor(ctx, tagsdb.UpdateTagColorParams{
			EntityID: row.EntityID,
			Color:    colorParam,
		})
		if err != nil {
			return fmt.Errorf("tag.UpdateByUUID update: %w", err)
		}

		ownerEntity, err := txCoreQ.GetEntityByID(ctx, row.OwnerID)
		if err != nil {
			return fmt.Errorf("tag.UpdateByUUID resolve owner: %w", err)
		}
		subjectEntity, err := txCoreQ.GetEntityByID(ctx, row.SubjectID)
		if err != nil {
			return fmt.Errorf("tag.UpdateByUUID resolve subject: %w", err)
		}

		result = hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, updated)

		after := colorSnapshot(updated.Color)
		return s.obs.Observe(ctx, tx, "update", "tag", &entityID, before, after)
	})
	if err != nil {
		return Tag{}, err
	}

	// 4. Post-commit observers — carry the post-update snapshot so that future
	// cache-invalidation or search-index-sync observers have the after-state.
	s.obs.ObserveAfterCommit(ctx, "update", "tag", &entityID, tagSnapshot(result))
	return result, nil
}

// UpdateTagValue updates the value of a tag identified by entity UUID.
// Validates that value is non-empty and at most 512 characters.
// Row-level access control (owner only, not subject) is enforced by the Authorizer.
func (s *TagService) UpdateTagValue(
	ctx context.Context,
	coreQ coredb.Querier,
	entityUUID uuid.UUID,
	in UpdateTagValueInput,
) (Tag, error) {
	// Validate value before any DB call.
	in.Value = strings.TrimSpace(in.Value)
	if in.Value == "" {
		return Tag{}, fmt.Errorf("%w: value is required", ErrInvalidInput)
	}
	if len(in.Value) > 512 {
		return Tag{}, fmt.Errorf("%w: value exceeds 512 characters", ErrInvalidInput)
	}

	// 1. Resolve the tag's entity UUID up front (pre-tx), mirroring
	// GetByUUID's pre-tx Resolve call: a genuine miss surfaces as
	// ErrForbidden (403) directly, before entering the tx.
	if _, err := s.entityResolver.Resolve(ctx, coreQ, entityUUID, "tag"); err != nil {
		return Tag{}, err
	}

	// 2. Authorize — authorize before fetching; use nil target (no entity ID yet).
	if err := s.az.Authorize(ctx, "update", nil); err != nil {
		return Tag{}, err
	}

	// 3. Mutate inside a transaction; observers participate in the same tx.
	var result Tag
	var entityID int64

	err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		txCoreQ := s.coreQuerier(tx)
		txTagQ := s.tagQuerier(tx)

		// Fetch the existing tag (before snapshot). entityResolver.Resolve
		// already confirmed the entity exists (pre-tx, above); a miss here
		// is a data-consistency case (entity resolved, tags row absent), so
		// it also masks to ErrForbidden, not ErrNotFound — masking-consistent
		// per the task's cross-cutting note, avoiding a residual gap.
		row, err := txTagQ.GetTagByEntityUUID(ctx, entityUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrForbidden
			}
			return fmt.Errorf("tag.UpdateTagValue fetch: %w", err)
		}

		entityID = row.EntityID
		before := valueSnapshot(row.Value)

		updated, err := txTagQ.UpdateTagValue(ctx, tagsdb.UpdateTagValueParams{
			EntityID: row.EntityID,
			Value:    in.Value,
		})
		if err != nil {
			return fmt.Errorf("tag.UpdateTagValue update: %w", err)
		}

		ownerEntity, err := txCoreQ.GetEntityByID(ctx, row.OwnerID)
		if err != nil {
			return fmt.Errorf("tag.UpdateTagValue resolve owner: %w", err)
		}
		subjectEntity, err := txCoreQ.GetEntityByID(ctx, row.SubjectID)
		if err != nil {
			return fmt.Errorf("tag.UpdateTagValue resolve subject: %w", err)
		}

		result = hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, updated)

		after := valueSnapshot(updated.Value)
		return s.obs.Observe(ctx, tx, "update", "tag", &entityID, before, after)
	})
	if err != nil {
		return Tag{}, err
	}

	// 4. Post-commit observers — carry the post-update snapshot.
	s.obs.ObserveAfterCommit(ctx, "update", "tag", &entityID, tagSnapshot(result))
	return result, nil
}

// DeleteByUUID removes a tag. The tags row is deleted; the entity row is
// archived (core-model exposes ArchiveEntity, not a hard DELETE).
// Row-level access control (owner only, not subject) is enforced by the
// Authorizer; the inline filter is no longer needed.
func (s *TagService) DeleteByUUID(
	ctx context.Context,
	coreQ coredb.Querier,
	entityUUID uuid.UUID,
) error {
	// 1. Resolve the tag's entity UUID up front (pre-tx), mirroring
	// GetByUUID's pre-tx Resolve call: a genuine miss surfaces as
	// ErrForbidden (403) directly, before entering the tx.
	if _, err := s.entityResolver.Resolve(ctx, coreQ, entityUUID, "tag"); err != nil {
		return err
	}

	// 2. Authorize.
	if err := s.az.Authorize(ctx, "delete", nil); err != nil {
		return err
	}

	// 3. Mutate inside a transaction; observers participate in the same tx.
	var entityID int64

	err := txhelper.Run(ctx, s.db, func(ctx context.Context, tx pgx.Tx) error {
		txTagQ := s.tagQuerier(tx)
		txCoreQ := s.coreQuerier(tx)

		// entityResolver.Resolve already confirmed the entity exists
		// (pre-tx, above); a miss here is a data-consistency case (entity
		// resolved, tags row absent), so it also masks to ErrForbidden, not
		// ErrNotFound — masking-consistent per the task's cross-cutting
		// note, avoiding a residual gap.
		row, err := txTagQ.GetTagByEntityUUID(ctx, entityUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrForbidden
			}
			return fmt.Errorf("tag.DeleteByUUID fetch: %w", err)
		}

		entityID = row.EntityID

		// Capture before snapshot for observation.
		ownerEntity, err := txCoreQ.GetEntityByID(ctx, row.OwnerID)
		if err != nil {
			return fmt.Errorf("tag.DeleteByUUID resolve owner: %w", err)
		}
		subjectEntity, err := txCoreQ.GetEntityByID(ctx, row.SubjectID)
		if err != nil {
			return fmt.Errorf("tag.DeleteByUUID resolve subject: %w", err)
		}
		beforeSnapshot := tagSnapshot(hydrateTag(row.Uuid, ownerEntity.Uuid, subjectEntity.Uuid, tagFromUUIDRow(row)))

		if err := txTagQ.DeleteTag(ctx, row.EntityID); err != nil {
			return fmt.Errorf("tag.DeleteByUUID delete tag: %w", err)
		}

		if err := txCoreQ.ArchiveEntity(ctx, entityUUID); err != nil {
			return fmt.Errorf("tag.DeleteByUUID archive entity: %w", err)
		}

		return s.obs.Observe(ctx, tx, "delete", "tag", &entityID, beforeSnapshot, nil)
	})
	if err != nil {
		return err
	}

	// 4. Post-commit observers — after is nil intentionally: the row no longer
	// exists, so there is no meaningful post-state to carry.
	s.obs.ObserveAfterCommit(ctx, "delete", "tag", &entityID, nil)
	return nil
}

// --- helpers ---

// hydrateTag converts internal DB rows to the service Tag type.
func hydrateTag(entityUUID, ownerUUID, subjectUUID uuid.UUID, t tagsdb.Tag) Tag {
	tag := Tag{
		EntityUUID:  entityUUID,
		OwnerUUID:   ownerUUID,
		SubjectUUID: subjectUUID,
		Purpose:     t.Purpose,
		Value:       t.Value,
	}
	if t.Color.Valid {
		c := t.Color.String
		tag.Color = &c
	}
	if t.CreatedAt.Valid {
		tag.CreatedAt = t.CreatedAt.Time
	}
	if t.UpdatedAt.Valid {
		tag.UpdatedAt = t.UpdatedAt.Time
	}
	return tag
}

// hydrateTagFromSearchRow converts a SearchTagsRow to the service Tag type.
// The tag entity UUID comes directly from the SQL JOIN on entities.
func hydrateTagFromSearchRow(entityUUID, ownerUUID, subjectUUID uuid.UUID, r tagsdb.SearchTagsRow) Tag {
	tag := Tag{
		EntityUUID:  entityUUID,
		OwnerUUID:   ownerUUID,
		SubjectUUID: subjectUUID,
		Purpose:     r.Purpose,
		Value:       r.Value,
	}
	if r.Color.Valid {
		c := r.Color.String
		tag.Color = &c
	}
	if r.CreatedAt.Valid {
		tag.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		tag.UpdatedAt = r.UpdatedAt.Time
	}
	return tag
}

// hydrateTagFromListRow converts a ListTagsBySubjectEntityIDRow to the service Tag type.
// The tag entity UUID comes directly from the SQL JOIN on entities.
func hydrateTagFromListRow(entityUUID, ownerUUID, subjectUUID uuid.UUID, r tagsdb.ListTagsBySubjectEntityIDRow) Tag {
	tag := Tag{
		EntityUUID:  entityUUID,
		OwnerUUID:   ownerUUID,
		SubjectUUID: subjectUUID,
		Purpose:     r.Purpose,
		Value:       r.Value,
	}
	if r.Color.Valid {
		c := r.Color.String
		tag.Color = &c
	}
	if r.CreatedAt.Valid {
		tag.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		tag.UpdatedAt = r.UpdatedAt.Time
	}
	return tag
}

// tagFromUUIDRow converts a GetTagByEntityUUIDRow back into a Tag row shape for hydrateTag.
func tagFromUUIDRow(r tagsdb.GetTagByEntityUUIDRow) tagsdb.Tag {
	return tagsdb.Tag{
		EntityID:  r.EntityID,
		OwnerID:   r.OwnerID,
		SubjectID: r.SubjectID,
		Purpose:   r.Purpose,
		Value:     r.Value,
		Color:     r.Color,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// tagSnapshot builds an observation snapshot map from a Tag.
func tagSnapshot(t Tag) map[string]any {
	return map[string]any{
		"uuid":         t.EntityUUID.String(),
		"owner_uuid":   t.OwnerUUID.String(),
		"subject_uuid": t.SubjectUUID.String(),
		"purpose":      t.Purpose,
		"value":        t.Value,
		"color":        t.Color,
	}
}

// colorSnapshot builds a before/after snapshot for color-only changes.
func colorSnapshot(c pgtype.Text) map[string]any {
	if c.Valid {
		return map[string]any{"color": c.String}
	}
	return map[string]any{"color": nil}
}

// valueSnapshot builds a before/after snapshot for value-only changes.
func valueSnapshot(value string) map[string]any {
	return map[string]any{"value": value}
}

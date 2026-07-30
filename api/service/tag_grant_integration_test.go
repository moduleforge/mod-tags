//go:build integration

package service

// tag_grant_integration_test.go proves that the generic GrantTableGenerator
// (mod-core/api/authz/setup) produces a correct accessible_tag_ids_for_actor
// function when applied to real tag rows created through mod-tags' own
// TagService.Create -- exercising Phase 2's ownership cutover
// (TagService.Create -> coredb.CreateEntityWithOwner) end to end against a
// live Postgres, rather than synthetic/hand-seeded rows or SQL-string
// assertions. It is the "real data in mod-tags" proof
// phase-04-generator-verification exists to provide; the complementary
// phase-02 verify-cutover-and-retain-invariants task verifies via diff/grep
// that nothing was dropped or rewritten, but does not exercise the generic
// access function against live data the way this suite does.
//
// Run with (from mod-tags/api):
//
//	go test -tags=integration -p 1 -v ./service/...
//
// -p 1 is required: TestMain drops and recreates the tags_grant_verify_dev
// database, so concurrent runs against the same host would conflict.
//
// Host resolution: TAGS_DEV_PG_HOST overrides; otherwise the suite dials
// "localhost", then the users-module-postgres container's own Docker-network
// IP (resolved via `docker inspect`), then a documented last-resort fallback
// IP, and uses whichever candidate actually accepts a TCP connection. (Some
// sandboxes run the test process as a peer of the Docker network, where
// "localhost" resolves to the test process's own loopback rather than the
// container and the container IP is required; others publish the container
// port directly to the host's own loopback, where "localhost" is correct and
// the container-network IP is unreachable. Probing avoids hard-coding either
// assumption.)
//
// Composed schema: mod-tags does not check in a pre-built
// model/schema/migrations/ directory (it is a git-ignored build artifact of
// `make -C model compose`). TestMain instead assembles an ephemeral composed
// migrations directory at run time from mod-core's core migrations (1-99,
// located dynamically -- see resolveCoreMigrationsDir) plus mod-tags' own
// migrations (200-299, in this repo), mirroring what `make compose` would
// produce, without depending on that command having been run first or on any
// hard-coded, machine-specific path outside this repo.
//
// Grants-mechanism fixture: mod-core's GrantTableGenerator emits a body whose
// shared CTE prefix (ActorChain/WildcardAdmin/GrantedTarget/TargetChain)
// unconditionally references the grants, authz_operations,
// authz_actor_group_members, and authz_target_group_members tables,
// regardless of which resource slug is being generated for. Those tables are
// owned by the mod-authz module (migrations 0500+), which is out of this
// repo's scope to touch or depend on -- mod-tags' own vendored composed
// schema is deliberately just core (1-99) + its own (200-299). Postgres
// resolves every table reference in a LANGUAGE SQL function body at CREATE
// FUNCTION time (verified empirically: CREATE FUNCTION ... LANGUAGE sql
// referencing a nonexistent table fails immediately, not at call time), so
// setup.ApplyFuncs cannot even create accessible_tag_ids_for_actor without
// these tables existing -- this is required for every assertion in this
// file, not only the wildcard/target-chain-specific ones.
//
// createAuthzSupportSchema below recreates the minimal subset of that
// mod-authz schema directly via SQL (not a goose migration -- it is a test
// fixture, not a tracked schema change) with the same table/column shapes
// mod-authz/model/migrations/0500,0502,0504,0505 define. This mirrors the
// pattern the sibling phase-04-generator-verification mod-core-live-
// verification task documents for the authz_actor_group/authz_target_group
// slugs: seed the minimal local schema objects a peer module owns rather
// than reach into that module's repo. Cycle-prevention and member-type-check
// triggers are intentionally omitted -- they are mod-authz
// application-correctness features, not required for the generic
// access-function SQL to compile or execute correctly here.

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/moduleforge/core-api/authz/setup"
	"github.com/moduleforge/core-api/entity"
	"github.com/moduleforge/core-api/observer"
	"github.com/moduleforge/core-api/opctx"
	"github.com/moduleforge/core-api/types"
	coredb "github.com/moduleforge/core-model/db"
	tagsdb "github.com/moduleforge/tags-model/db"
)

// tagsGrantVerifyDB is the dedicated shadow database this suite drops,
// recreates, and migrates on every run.
const tagsGrantVerifyDB = "tags_grant_verify_dev"

// integrationPool and integrationSvcs are initialized once in TestMain and
// shared read-only across all tests in this file (serialized by -p 1 and by
// running as a single test binary; no test mutates the schema itself).
var (
	integrationPool *pgxpool.Pool
	integrationSvcs *Services
)

// readOpIDs mirrors stubOpResolver's fixed id set (mock_test.go) so raw SQL
// calls against accessible_tag_ids_for_actor use the same op_ids closure the
// wired Services would compute via opRes.SatisfiedBy("read").
var readOpIDs = []int32{1, 2, 3, 4, 5, 6, 7}

// ----------------------------------------------------------------------------
// TestMain
// ----------------------------------------------------------------------------

func TestMain(m *testing.M) {
	if err := checkIntegrationPrerequisites(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: skipping tag grant tests — %v\n", err)
		// Exit 0 so `go test` reports the package as skipped, not failed.
		os.Exit(0)
	}

	root := repoRoot()
	coreDir, err := resolveCoreMigrationsDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: skipping tag grant tests — %v\n", err)
		os.Exit(0)
	}
	tagsDir := filepath.Join(root, "model", "migrations")

	pgHost, ok := resolvePostgresHost()
	if !ok {
		fmt.Fprintf(os.Stderr, "integration: skipping tag grant tests — no reachable Postgres host candidate "+
			"(tried TAGS_DEV_PG_HOST override, localhost, the users-module-postgres docker-network IP, "+
			"and the documented fallback IP)\n")
		os.Exit(0)
	}

	if err := resetTagsGrantVerifyDB(pgHost); err != nil {
		fmt.Fprintf(os.Stderr, "integration: DB reset failed: %v\n", err)
		os.Exit(1)
	}

	migDir, cleanup, err := composedMigrationsDir(coreDir, tagsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: compose migrations dir: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	if err := applyMigrations(pgHost, migDir); err != nil {
		fmt.Fprintf(os.Stderr, "integration: apply migrations: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	dsn := fmt.Sprintf("postgres://users:users@%s:5432/%s?sslmode=disable", pgHost, tagsGrantVerifyDB)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: open pool: %v\n", err)
		os.Exit(1)
	}
	integrationPool = pool

	if err := createAuthzSupportSchema(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "integration: %v\n", err)
		pool.Close()
		os.Exit(1)
	}
	if err := seedGroupTypes(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "integration: seed group types: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	coreQ := coredb.New(pool)
	tagQ := tagsdb.New(pool)

	resolver, err := types.New(ctx, coreQ)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: type resolver: %v\n", err)
		pool.Close()
		os.Exit(1)
	}

	// allowAllAuthz / newStubOpResolver are defined in mock_test.go (same
	// package, no build tag -- compiled into both the default and
	// integration test binaries). Authorization gating of Create/ListBySubject
	// is not what this suite proves; the generic access function applied
	// separately via setup.ApplyFuncs and queried directly is.
	integrationSvcs = New(coreQ, tagQ, pool, allowAllAuthz{}, newStubOpResolver(), observer.NewObserverGroup(), resolver, entity.NewResolver())

	code := m.Run()
	pool.Close()
	os.Exit(code)
}

// ----------------------------------------------------------------------------
// Prerequisites / host resolution / migrations plumbing
// ----------------------------------------------------------------------------

// checkIntegrationPrerequisites verifies docker, goose, and a discoverable
// mod-core migrations directory are all available. Any failure here means
// the test degrades gracefully (skip), not a hard failure.
func checkIntegrationPrerequisites() error {
	cmd := exec.Command("docker", "inspect", "--format={{.State.Running}}", "users-module-postgres")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docker inspect: %w", err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("container users-module-postgres is not running")
	}
	if _, err := exec.LookPath("goose"); err != nil {
		return fmt.Errorf("goose not in PATH: %w", err)
	}
	if _, err := resolveCoreMigrationsDir(repoRoot()); err != nil {
		return err
	}
	return nil
}

// repoRoot returns this repo's root directory (the parent of api/ and
// model/), derived from this file's own path so it is correct whether this
// is a normal checkout or a nested worktree checkout.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// file = <repoRoot>/api/service/tag_grant_integration_test.go
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// resolveCoreMigrationsDir locates mod-core's model/migrations directory.
// CORE_MODEL_MIGRATIONS_DIR overrides explicitly. Otherwise, mod-tags and
// mod-core are sibling repos under a common aggregate root
// (.../moduleforge/{mod-tags,mod-core}); this walks upward from this repo's
// root looking for that sibling. That works for a normal checkout (the
// sibling is one level up) and for a worktree checkout nested several levels
// deeper (.../mod-tags/worktree/<branch>/), without hard-coding an absolute,
// machine-specific path anywhere in this file.
func resolveCoreMigrationsDir(root string) (string, error) {
	if d := os.Getenv("CORE_MODEL_MIGRATIONS_DIR"); d != "" {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d, nil
		}
		return "", fmt.Errorf("CORE_MODEL_MIGRATIONS_DIR=%q is not a directory", d)
	}

	dir := root
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "mod-core", "model", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf(
		"could not locate mod-core/model/migrations above %s (checked 8 ancestor levels); "+
			"set CORE_MODEL_MIGRATIONS_DIR to override", root)
}

// composedMigrationsDir builds an ephemeral directory containing every core
// (1-99) and mod-tags-own (200-299) *.sql migration file, mirroring
// model/Makefile's `compose` target's output but resolved and assembled at
// test run time. The caller must invoke the returned cleanup func.
func composedMigrationsDir(coreDir, tagsDir string) (dir string, cleanup func(), err error) {
	tmp, err := os.MkdirTemp("", "tags-grant-verify-migrations-*")
	if err != nil {
		return "", nil, fmt.Errorf("mkdir temp: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }

	for _, src := range []string{coreDir, tagsDir} {
		matches, globErr := filepath.Glob(filepath.Join(src, "*.sql"))
		if globErr != nil {
			cleanup()
			return "", nil, fmt.Errorf("glob %s: %w", src, globErr)
		}
		for _, m := range matches {
			data, readErr := os.ReadFile(m)
			if readErr != nil {
				cleanup()
				return "", nil, fmt.Errorf("read %s: %w", m, readErr)
			}
			if writeErr := os.WriteFile(filepath.Join(tmp, filepath.Base(m)), data, 0o644); writeErr != nil {
				cleanup()
				return "", nil, fmt.Errorf("write %s: %w", m, writeErr)
			}
		}
	}
	return tmp, cleanup, nil
}

// resolvePostgresHost returns the host to use for connecting to the
// users-module-postgres container, and whether any candidate is actually
// reachable. TAGS_DEV_PG_HOST always wins when set (no reachability check
// applied to an explicit override). Absent that, candidates are tried in
// order and each is verified with a real TCP dial on port 5432 rather than
// assumed:
//
//  1. "localhost" -- correct when the container publishes its port straight
//     to the host's own loopback (observed in this environment: Docker
//     Desktop for macOS. The docker-network IP times out from here).
//  2. The container's Docker-network IP via `docker inspect` -- correct in
//     sandboxes where the test process itself runs as a peer of the Docker
//     network, so "localhost" resolves to the test process's own loopback
//     instead of the container (the assumption the mod-authz/mod-audit/
//     mod-users integration-test precedents bake in).
//  3. A documented last-resort fallback IP from that same precedent.
//
// Probing avoids hard-coding either environment's assumption. If no
// candidate dials successfully, ok is false and the caller should skip
// gracefully rather than proceed to a hard failure.
func resolvePostgresHost() (host string, ok bool) {
	if h := os.Getenv("TAGS_DEV_PG_HOST"); h != "" {
		return h, true
	}

	candidates := []string{"localhost"}
	if ip := dockerNetworkIP("users-module-postgres"); ip != "" {
		candidates = append(candidates, ip)
	}
	candidates = append(candidates, "172.23.0.3")

	for _, c := range candidates {
		if canDial(c, 5432) {
			return c, true
		}
	}
	return "", false
}

// canDial reports whether a TCP connection to host:port succeeds within a
// short timeout.
func canDial(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// dockerNetworkIP returns the container's Docker-network IP, or "" if it
// cannot be determined.
func dockerNetworkIP(container string) string {
	cmd := exec.Command("docker", "inspect",
		"--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		container)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// resetTagsGrantVerifyDB drops and recreates tags_grant_verify_dev.
func resetTagsGrantVerifyDB(pgHost string) error {
	ctx := context.Background()
	adminURL := fmt.Sprintf("postgres://users:users@%s:5432/postgres?sslmode=disable", pgHost)

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect admin: %w", err)
	}
	defer conn.Close(ctx)

	for _, stmt := range []string{
		"DROP DATABASE IF EXISTS " + tagsGrantVerifyDB,
		"CREATE DATABASE " + tagsGrantVerifyDB,
	} {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

// applyMigrations runs `goose up` against tags_grant_verify_dev using the
// composed migrations directory.
func applyMigrations(pgHost, migrationsDir string) error {
	dsn := fmt.Sprintf("postgres://users:users@%s:5432/%s?sslmode=disable", pgHost, tagsGrantVerifyDB)
	cmd := exec.Command("goose", "-dir", migrationsDir, "postgres", dsn, "up") //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("goose up: %w\n%s", err, out)
	}
	return nil
}

// createAuthzSupportSchema recreates, directly via SQL (see file-level doc
// comment), the minimal subset of mod-authz's grants-mechanism schema that
// mod-core's GrantTableGenerator's shared CTE prefix unconditionally
// references.
func createAuthzSupportSchema(ctx context.Context, pool *pgxpool.Pool) error {
	const ddl = `
CREATE TABLE authz_operations (
    id   INT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE
);

INSERT INTO authz_operations (id, slug) VALUES
    (1, 'read'), (2, 'sread'), (3, 'list'), (4, 'update'),
    (5, 'delete'), (6, 'supdate'), (7, 'manage'), (13, 'create');

CREATE TABLE grants (
    id           BIGSERIAL PRIMARY KEY,
    actor_id     BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    operation_id INT    NOT NULL REFERENCES authz_operations(id),
    target_id    BIGINT REFERENCES entities(id) ON DELETE CASCADE
);

CREATE TABLE authz_actor_group_members (
    group_id  BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    member_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, member_id)
);

CREATE TABLE authz_target_group_members (
    group_id  BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    member_id BIGINT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, member_id)
);
`
	if _, err := pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("createAuthzSupportSchema: %w", err)
	}
	return nil
}

// seedGroupTypes registers the 'authz_actor_group' and 'authz_target_group'
// type slugs directly (mirroring mod-authz/model/migrations/0501,0503),
// mirroring the treatment the sibling phase-04-generator-verification
// mod-core-live-verification task documents for these same two slugs: a
// local, minimal fixture registration rather than a dependency on
// mod-authz's own migrations.
func seedGroupTypes(ctx context.Context, pool *pgxpool.Pool) error {
	const ddl = `
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT 'authz_actor_group', id, true, 'Authorization Actor Group (test fixture)',
       'Local minimal fixture registration for grant-chain integration testing.'
FROM types WHERE slug = 'entity';

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT 'authz_target_group', id, true, 'Authorization Target Group (test fixture)',
       'Local minimal fixture registration for grant-chain integration testing.'
FROM types WHERE slug = 'entity';
`
	_, err := pool.Exec(ctx, ddl)
	return err
}

// ----------------------------------------------------------------------------
// Seed helpers
// ----------------------------------------------------------------------------

// seedActor creates a natural_person entity (full CTI chain: entities ->
// legal_entities -> natural_persons) and returns its internal entity ID. The
// entities_owner_self_default trigger (mod-core 0013_entity_ownership.sql)
// sets entities.owner_id = id automatically for natural_person inserts.
func seedActor(ctx context.Context, pool *pgxpool.Pool, name string) (int64, error) {
	var npTypeID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM types WHERE slug = 'natural_person'`).Scan(&npTypeID); err != nil {
		return 0, fmt.Errorf("seedActor(%s): resolve natural_person type: %w", name, err)
	}

	var entityID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO entities (fundamental_type_id) VALUES ($1) RETURNING id`, npTypeID,
	).Scan(&entityID); err != nil {
		return 0, fmt.Errorf("seedActor(%s): insert entity: %w", name, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO legal_entities (entity_id) VALUES ($1)`, entityID); err != nil {
		return 0, fmt.Errorf("seedActor(%s): insert legal_entity: %w", name, err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO natural_persons (entity_id, given_name) VALUES ($1, $2)`, entityID, name,
	); err != nil {
		return 0, fmt.Errorf("seedActor(%s): insert natural_person: %w", name, err)
	}
	return entityID, nil
}

// seedGroupEntity inserts a bare entities row of the given (already
// registered via seedGroupTypes) fixture type slug -- no subtype table
// required, since the generic access function's ActorChain/TargetChain CTEs
// only ever join authz_actor_group_members/authz_target_group_members by id,
// never by a group subtype table.
func seedGroupEntity(ctx context.Context, pool *pgxpool.Pool, typeSlug string) (int64, error) {
	var typeID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM types WHERE slug = $1`, typeSlug).Scan(&typeID); err != nil {
		return 0, fmt.Errorf("seedGroupEntity(%s): resolve type: %w", typeSlug, err)
	}
	var entityID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO entities (fundamental_type_id) VALUES ($1) RETURNING id`, typeID,
	).Scan(&entityID); err != nil {
		return 0, fmt.Errorf("seedGroupEntity(%s): insert entity: %w", typeSlug, err)
	}
	return entityID, nil
}

// ----------------------------------------------------------------------------
// Query / assertion helpers
// ----------------------------------------------------------------------------

func mustEntityUUID(ctx context.Context, t *testing.T, pool *pgxpool.Pool, entityID int64) uuid.UUID {
	t.Helper()
	var u uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT uuid FROM entities WHERE id = $1`, entityID).Scan(&u); err != nil {
		t.Fatalf("mustEntityUUID(%d): %v", entityID, err)
	}
	return u
}

func mustEntityID(ctx context.Context, t *testing.T, pool *pgxpool.Pool, u uuid.UUID) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, `SELECT id FROM entities WHERE uuid = $1`, u).Scan(&id); err != nil {
		t.Fatalf("mustEntityID(%s): %v", u, err)
	}
	return id
}

// mustAccessibleTagIDs calls the real, generator-produced
// accessible_tag_ids_for_actor(...) function directly and returns the result
// as a set.
func mustAccessibleTagIDs(ctx context.Context, t *testing.T, pool *pgxpool.Pool, actorID int64) map[int64]bool {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT entity_id FROM accessible_tag_ids_for_actor($1, $2)`, actorID, readOpIDs)
	if err != nil {
		t.Fatalf("accessible_tag_ids_for_actor(%d): %v", actorID, err)
	}
	defer rows.Close()

	got := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan accessible_tag_ids_for_actor row: %v", err)
		}
		got[id] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("accessible_tag_ids_for_actor rows: %v", err)
	}
	return got
}

func assertAccessible(t *testing.T, set map[int64]bool, id int64) {
	t.Helper()
	if !set[id] {
		t.Errorf("expected entity id %d to be accessible, got set %v", id, set)
	}
}

func assertNotAccessible(t *testing.T, set map[int64]bool, id int64) {
	t.Helper()
	if set[id] {
		t.Errorf("expected entity id %d NOT to be accessible, got set %v", id, set)
	}
}

// assertOwnerInvariant confirms both halves of the Phase 2 cutover for one
// created tag: entities.owner_id equals the actorEntityID passed to Create
// (proves SetEntityOwner/CreateEntityWithOwner persisted the centralized
// owner from the real transaction), and entities.owner_id equals the local
// tags.owner_id (the immutable-denormalized-copy invariant from
// plan/notes/local-owner-column-fate.md).
func assertOwnerInvariant(ctx context.Context, t *testing.T, pool *pgxpool.Pool, tagEntityID, wantOwner int64) {
	t.Helper()
	var entitiesOwner, tagsOwner int64
	err := pool.QueryRow(ctx,
		`SELECT e.owner_id, t.owner_id FROM entities e JOIN tags t ON t.entity_id = e.id WHERE e.id = $1`,
		tagEntityID,
	).Scan(&entitiesOwner, &tagsOwner)
	if err != nil {
		t.Fatalf("assertOwnerInvariant(%d): %v", tagEntityID, err)
	}
	if entitiesOwner != wantOwner {
		t.Errorf("entities.owner_id = %d, want %d (the actorEntityID passed to Create)", entitiesOwner, wantOwner)
	}
	if entitiesOwner != tagsOwner {
		t.Errorf("entities.owner_id (%d) != tags.owner_id (%d); immutable-denormalized-copy invariant violated",
			entitiesOwner, tagsOwner)
	}
}

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

// TestTagGrantIntegration creates real tags through TagService.Create,
// applies the real GrantTableGenerator, and asserts accessible_tag_ids_for_actor
// behaves correctly against live data for every case Requirement 6 of
// phase-04-generator-verification/003-mod-tags-live-verification.md names.
func TestTagGrantIntegration(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool
	coreQ := coredb.New(pool)
	tagQ := tagsdb.New(pool)

	actorA, err := seedActor(ctx, pool, "actor-a")
	if err != nil {
		t.Fatalf("seed actor A: %v", err)
	}
	actorB, err := seedActor(ctx, pool, "actor-b")
	if err != nil {
		t.Fatalf("seed actor B: %v", err)
	}
	subjectS1, err := seedActor(ctx, pool, "subject-s1")
	if err != nil {
		t.Fatalf("seed subject S1: %v", err)
	}
	subjectS2, err := seedActor(ctx, pool, "subject-s2")
	if err != nil {
		t.Fatalf("seed subject S2: %v", err)
	}

	subjectS1UUID := mustEntityUUID(ctx, t, pool, subjectS1)
	subjectS2UUID := mustEntityUUID(ctx, t, pool, subjectS2)
	actorBUUID := mustEntityUUID(ctx, t, pool, actorB) // B doubles as tag3's subject

	// Three real tags via the actual Create call path (Requirement 4):
	//   tag1: owner A, subject S1
	//   tag2: owner B, subject S2
	//   tag3: owner A, subject B -- A owns it, B is only the subject; this is
	//         the case that must now be DENIED to B post-refactor.
	tag1, err := integrationSvcs.Tag.Create(opctx.WithActor(ctx, actorA), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectS1UUID, Purpose: "grant-verify-1", Value: "v1",
	})
	if err != nil {
		t.Fatalf("Create tag1: %v", err)
	}
	tag2, err := integrationSvcs.Tag.Create(opctx.WithActor(ctx, actorB), coreQ, CreateTagInput{
		SubjectEntityUUID: subjectS2UUID, Purpose: "grant-verify-2", Value: "v2",
	})
	if err != nil {
		t.Fatalf("Create tag2: %v", err)
	}
	tag3, err := integrationSvcs.Tag.Create(opctx.WithActor(ctx, actorA), coreQ, CreateTagInput{
		SubjectEntityUUID: actorBUUID, Purpose: "grant-verify-3", Value: "v3",
	})
	if err != nil {
		t.Fatalf("Create tag3: %v", err)
	}

	tag1ID := mustEntityID(ctx, t, pool, tag1.EntityUUID)
	tag2ID := mustEntityID(ctx, t, pool, tag2.EntityUUID)
	tag3ID := mustEntityID(ctx, t, pool, tag3.EntityUUID)

	// Requirement 4 + the immutable-denormalized-copy Validation check: for
	// every created tag, entities.owner_id equals the real actorEntityID
	// passed to Create, and equals the local tags.owner_id.
	t.Run("owner_persisted_to_centralized_entities_table", func(t *testing.T) {
		assertOwnerInvariant(ctx, t, pool, tag1ID, actorA)
		assertOwnerInvariant(ctx, t, pool, tag2ID, actorB)
		assertOwnerInvariant(ctx, t, pool, tag3ID, actorA)
	})

	// Requirement 5: apply the real generic access function.
	if err := setup.ApplyFuncs(ctx, pool, setup.NewGrantTableGenerator(), []string{"tag"}); err != nil {
		t.Fatalf("setup.ApplyFuncs: %v", err)
	}

	t.Run("owner_sees_own_tags", func(t *testing.T) {
		ids := mustAccessibleTagIDs(ctx, t, pool, actorA)
		assertAccessible(t, ids, tag1ID)
		assertAccessible(t, ids, tag3ID)
		assertNotAccessible(t, ids, tag2ID) // A does not own tag2
	})

	t.Run("subject_denied_non_owner_no_longer_sees_tag", func(t *testing.T) {
		// B owns tag2 (sees it) but is only the SUBJECT of tag3, not its
		// owner -- the dropped subject_id access predicate means B must NOT
		// see tag3. This is the explicit, user-approved behavioral
		// regression this task exists to prove live, not just by SQL-string
		// absence (Phase 1's unit tests already cover that).
		ids := mustAccessibleTagIDs(ctx, t, pool, actorB)
		assertAccessible(t, ids, tag2ID)
		assertNotAccessible(t, ids, tag3ID)
		assertNotAccessible(t, ids, tag1ID)
	})

	t.Run("wildcard_grant_actor_sees_all_seeded_tags", func(t *testing.T) {
		w, err := seedActor(ctx, pool, "wildcard-admin")
		if err != nil {
			t.Fatalf("seed wildcard actor: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO grants (actor_id, operation_id, target_id) VALUES ($1, $2, NULL)`,
			w, 1, // operation_id 1 = "read", included in readOpIDs
		); err != nil {
			t.Fatalf("insert wildcard grant: %v", err)
		}

		ids := mustAccessibleTagIDs(ctx, t, pool, w)
		assertAccessible(t, ids, tag1ID)
		assertAccessible(t, ids, tag2ID)
		assertAccessible(t, ids, tag3ID)
	})

	t.Run("target_chain_granted_actor_sees_exactly_granted_tag", func(t *testing.T) {
		c, err := seedActor(ctx, pool, "chain-actor")
		if err != nil {
			t.Fatalf("seed chain actor: %v", err)
		}
		ag, err := seedGroupEntity(ctx, pool, "authz_actor_group")
		if err != nil {
			t.Fatalf("seed actor group: %v", err)
		}
		tg, err := seedGroupEntity(ctx, pool, "authz_target_group")
		if err != nil {
			t.Fatalf("seed target group: %v", err)
		}

		if _, err := pool.Exec(ctx,
			`INSERT INTO authz_actor_group_members (group_id, member_id) VALUES ($1, $2)`, ag, c,
		); err != nil {
			t.Fatalf("insert actor group member: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO authz_target_group_members (group_id, member_id) VALUES ($1, $2)`, tg, tag1ID,
		); err != nil {
			t.Fatalf("insert target group member: %v", err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO grants (actor_id, operation_id, target_id) VALUES ($1, $2, $3)`, ag, 1, tg,
		); err != nil {
			t.Fatalf("insert target-chain grant: %v", err)
		}

		ids := mustAccessibleTagIDs(ctx, t, pool, c)
		assertAccessible(t, ids, tag1ID)
		assertNotAccessible(t, ids, tag2ID)
		assertNotAccessible(t, ids, tag3ID)
	})

	t.Run("subject_id_still_queryable_via_non_auth_list_path", func(t *testing.T) {
		// subject_id itself is retained as a data column and filter, only its
		// use as an ACCESS predicate was dropped (see subtest above): owner A
		// listing by subject S1 must still return tag1 via the real,
		// non-mocked ListBySubject path -- proving the column and its
		// non-authorization uses are untouched by the predicate drop.
		results, err := integrationSvcs.Tag.ListBySubject(
			opctx.WithActor(ctx, actorA), coreQ, tagQ, subjectS1UUID, nil, Pagination{},
		)
		if err != nil {
			t.Fatalf("ListBySubject: %v", err)
		}
		found := false
		for _, r := range results {
			if r.EntityUUID == tag1.EntityUUID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("want tag1 (%s) in ListBySubject(S1) results for owner A, got %d results", tag1.EntityUUID, len(results))
		}
	})
}

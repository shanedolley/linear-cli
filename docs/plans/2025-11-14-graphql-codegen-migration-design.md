# GraphQL Code Generation Migration Design

**Date:** 2025-11-14
**Status:** Approved
**Author:** Architecture Review

## Problem Statement

The current linctl codebase maintains 1515 lines of hand-written GraphQL types and queries in `pkg/api/queries.go`. As Linear's API evolves and new fields are added, these types must be manually updated, leading to:

- Maintenance burden when Linear adds/changes fields
- Risk of runtime errors from type mismatches
- Difficulty keeping up with Linear's API evolution

## Goals

1. **Maintainability**: Automatic type generation from Linear's schema
2. **Performance**: Maintain current CLI performance (13MB binary, instant startup, low memory)
3. **Type Safety**: Catch API changes at compile-time, not runtime
4. **Low Risk**: Incremental migration without breaking existing functionality

## Decision: Go + genqlient Code Generation

After evaluating three options (Go with codegen, Rust rewrite, TypeScript rewrite), we chose to keep Go and add GraphQL code generation using `genqlient`.

### Why This Approach

**Performance maintained:**
- Current: 13MB binary, ~5ms startup, ~10MB memory
- Alternative (Rust): Marginal improvements (~20%) for 300% more effort
- Alternative (TypeScript): 50-100MB memory, 100-300ms startup - unsuitable for CLI

**Maintainability improved:**
- Linear schema updates → re-run generation → compiler errors show what changed
- No manual type maintenance
- 90% of TypeScript's codegen benefits with Go's performance

**Risk minimized:**
- Incremental migration (one entity at a time)
- Existing CLI structure preserved
- Each phase is independently testable and committable

## Architecture Changes

### New File Structure

```
pkg/api/
├── client.go              # Unchanged - HTTP client wrapper
├── genqlient.yaml         # NEW - genqlient configuration
├── schema.graphql         # NEW - Linear's GraphQL schema
├── operations/            # NEW - GraphQL operation definitions
│   ├── issues.graphql
│   ├── projects.graphql
│   ├── teams.graphql
│   ├── users.graphql
│   └── comments.graphql
└── generated/             # NEW - auto-generated Go code
    └── generated.go
```

### What Gets Generated

`genqlient` reads `.graphql` operation files and generates:
- Type-safe Go structs matching query responses
- Functions to execute queries with proper parameters
- Automatic marshaling/unmarshaling

### What Stays Manual

- CLI command structure (`cmd/*.go`)
- Output formatting (`pkg/output/`)
- Authentication flow (`pkg/auth/`)
- HTTP client wrapper (`pkg/api/client.go`)

## Migration Strategy

### Phase 1: Setup (Day 1)
1. Add `genqlient` dependency: `go get github.com/Khan/genqlient`
2. Download Linear's schema via introspection query
3. Create `genqlient.yaml` configuration
4. Set up `generated/` directory structure

### Phase 2: First Entity - Issues (Day 1-2)
1. Create `operations/issues.graphql` with Issue queries
2. Run code generation
3. Create adapter layer in `pkg/api/adapter.go` to maintain existing function signatures
4. Update `cmd/issue.go` to use adapters
5. Test: `make test` must pass
6. Commit: "Migrate Issues to genqlient"

### Phase 3: Remaining Entities (Day 3-4)
Repeat Phase 2 pattern for:
- Projects (1-2 hours)
- Teams (1-2 hours)
- Users (1-2 hours)
- Comments (1-2 hours)

Each entity is a separate commit for easy rollback.

### Phase 4: Cleanup (Day 4)
1. Delete `pkg/api/queries.go` entirely (1515 lines removed)
2. Evaluate adapter layer - remove if generated functions fit well, keep if they provide value
3. Update `CLAUDE.md` with new architecture documentation
4. Final commit: "Complete genqlient migration"

## Rollback Safety

- Each phase is a separate Git commit
- Commands continue working throughout migration
- Can stop at any phase and have a functional CLI
- If issues arise, revert to previous commit

## Schema Updates (Future)

When Linear updates their API:

```bash
# 1. Download new schema
./scripts/update-schema.sh

# 2. Re-run generation
go generate ./...

# 3. Compiler shows what broke
go build  # Will fail with compile errors where types changed

# 4. Fix the errors (compiler guides you)
# 5. Test and commit
```

## Success Criteria

- [ ] All existing commands work identically
- [ ] Smoke tests pass (`make test`)
- [ ] Binary size remains under 15MB
- [ ] No runtime performance regression
- [ ] `pkg/api/queries.go` deleted
- [ ] Documentation updated

## Non-Goals

- Rewriting CLI command structure
- Changing output formats
- Adding new Linear API features (do this after migration)
- Performance optimization (current performance is excellent)

## Estimated Timeline

- **Day 1**: Setup + Issues migration (4-6 hours)
- **Day 2**: Issues completion + Projects start (4 hours)
- **Day 3**: Teams + Users migration (4 hours)
- **Day 4**: Comments + cleanup + documentation (3-4 hours)

**Total**: 15-18 hours of focused work over 4 days

## Future Enhancements

After migration completes:
1. Add CI job to check for schema updates weekly
2. Consider generating operation files from Linear's schema introspection
3. Add more Linear API entities (Cycles, Initiatives, etc.)
4. Explore partial query generation for performance optimization

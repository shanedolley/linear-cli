# Direct genqlient Types - Remove Adapter Layer

**Date:** 2025-11-14
**Status:** Design Approved
**Author:** Architecture Review

## Problem Statement

The adapter layer converts between generated types and legacy types, adding 600 lines of maintenance burden. Commands use `map[string]interface{}` for filters, losing compile-time type safety—the primary benefit of genqlient.

**Current flow:**
```
Commands → map filters → Adapters → Generated types → GraphQL API
Commands ← Legacy types ← Adapters ← Generated types ← GraphQL API
```

**Desired flow:**
```
Commands → Typed filters → Generated types → GraphQL API
Commands ← Generated types ← GraphQL API
```

## Goals

1. **Eliminate maintenance burden** - Delete 600 lines of adapter/conversion code
2. **Achieve full type safety** - Commands build typed filters, get compile-time validation
3. **Preserve functionality** - All commands work identically, all 39 smoke tests pass
4. **Incremental migration** - Low-risk phases, test at each step

## Non-Goals

- Changing command-line interface (flags, arguments, help text)
- Modifying output formats (table, plaintext, JSON)
- Adding new features
- Optimizing performance beyond removing conversion overhead

## Migration Strategy

### Three Phases

**Phase 1: Read Operations** (This design)
- Commands: `list`, `get`, `search` for all entities
- Scope: 8 commands, ~400 lines of command code
- Risk: Low (read-only, can't corrupt data)
- Tests: All 39 smoke tests cover these operations

**Phase 2: Write Operations** (Future)
- Commands: `create`, `update`, `delete`
- Scope: More complex (map inputs → typed structs)
- Risk: Medium (mutations require careful testing)

**Phase 3: Cleanup** (Future)
- Delete `adapter.go` (~1600 lines)
- Delete unused types from `types.go`
- Update documentation

This design focuses on **Phase 1: Read Operations**.

## Phase 1: Technical Design

### Commands Affected

**Issues (3 commands):**
- `issue list` - Most used command, filters + pagination
- `issue get` - Simplest, single ID lookup
- `issue search` - Similar to list, adds search term

**Projects (2 commands):**
- `project list`
- `project get`

**Teams (3 commands):**
- `team list`
- `team get`
- `team members`

**Users (2 commands):**
- `user list`
- `user whoami`

**Comments (1 command):**
- `comment list`

### Filter Building Pattern

**Current (map-based):**
```go
filter := map[string]interface{}{}
if assignee != "" {
    filter["assignee"] = map[string]interface{}{
        "id": map[string]interface{}{"eq": assigneeID},
    }
}
```

**Problems:**
- No compile-time validation
- Typos discovered at runtime
- No IDE autocomplete
- Schema changes missed until deployment

**New (typed):**
```go
filter := IssueFilter{}
if assignee != "" {
    filter.Assignee = &NullableUserFilter{
        Id: &IDComparator{Eq: &assigneeID},
    }
}
```

**Benefits:**
- Typos = compile errors
- IDE autocomplete works
- Schema changes caught at build time
- Self-documenting (types show structure)

**Helper pattern for readability:**
```go
// In each command file
func stringEq(val string) *StringComparator {
    return &StringComparator{Eq: &val}
}

// Usage
filter.Team = &NullableTeamFilter{Key: stringEq(teamKey)}
```

### Response Type Handling

**Current:**
```go
issues, err := client.GetIssues(ctx, filter, limit, "", orderBy)
// issues is *Issues with nodes []Issue

for _, issue := range issues.Nodes {
    row = append(row, issue.Identifier, issue.Title)
}
```

**New:**
```go
resp, err := ListIssues(ctx, client, filter, &limit, nil, orderBy)
// resp is *ListIssuesResponse

for _, node := range resp.Issues.Nodes {
    fields := node.IssueListFields
    row = append(row, fields.Identifier, fields.Title)
}
```

**Key differences:**
1. Response is nested: `resp.Issues.Nodes` not `issues.Nodes`
2. Node types embed fragments: `IssueListFields`, `IssueDetailFields`
3. Field access through fragments: `node.IssueListFields.Title`

### Output Layer Changes

**JSON output:** Works unchanged (generated types serialize correctly)

**Table output:** Update field access patterns
```go
// Before
for _, issue := range issues.Nodes {
    row = append(row, issue.Identifier, issue.Title, issue.State.Name)
}

// After
for _, node := range resp.Issues.Nodes {
    f := node.IssueListFields // shorter variable for readability
    stateName := ""
    if f.State != nil {
        stateName = f.State.Name
    }
    row = append(row, f.Identifier, f.Title, stateName)
}
```

**Plaintext output:** Same pattern as table

### Error Handling

**Generated functions return clean errors:**
```go
resp, err := ListIssues(ctx, client, filter, &limit, nil, orderBy)
if err != nil {
    output.Error(fmt.Sprintf("Failed to list issues: %v", err), plaintext, jsonOut)
    os.Exit(1)
}
```

**Error types shift:**
- **Compile-time:** Typos, wrong types, invalid enum values
- **Runtime:** Invalid IDs, permissions, business logic

This is the desired outcome—catch errors earlier.

### Nil Checking

Generated types use pointers (nullable in GraphQL). Check before dereferencing:

```go
stateName := "No State"
if node.IssueListFields.State != nil {
    stateName = node.IssueListFields.State.Name
}
```

Pattern: provide sensible defaults for nil values.

### Pagination Handling

Generated functions use pointer parameters:

```go
var limitPtr *int
if limit > 0 {
    limitPtr = &limit
}

var afterPtr *string
if after != "" {
    afterPtr = &after
}

resp, err := ListIssues(ctx, client, filter, limitPtr, afterPtr, orderBy)
```

Standard pattern—apply to all list commands.

## Migration Execution

### Order of Migration

**1. `issue get` (Simplest)**
- No filters, no pagination
- ID lookup → single issue
- Proves basic pattern

**2. `issue list` (Core pattern)**
- Filters + pagination
- Most commonly used
- Tests full pattern complexity

**3. `issue search`**
- Reuses filter pattern
- Adds search term parameter

**4. Other entities**
- Projects, Teams, Users, Comments
- Apply patterns from issues
- Each entity: 2-3 commands

### Per-Command Changes

**Modified file:** `cmd/issue.go` (or project.go, team.go, etc.)

**Changes:**
1. Import generated types
2. Build typed filters
3. Call generated functions directly
4. Update field access for output
5. Add nil checks

**Unchanged files:**
- `pkg/api/adapter.go` - Kept for unmigrated commands
- `pkg/api/types.go` - Kept for unmigrated commands
- `pkg/api/generated.go` - Already complete
- `pkg/output/output.go` - Works with any JSON-serializable type

### Testing After Each Command

**Per command:**
```bash
# Build verification
make build

# Specific command test
./lincli issue get SD-37 --json
./lincli issue list --limit 5

# Full suite
./smoke_test.sh
```

**Expected:** All tests pass, identical output to pre-migration.

## Success Criteria - Phase 1

Phase 1 completes when:

- ✅ All 8 read commands migrated
- ✅ All 39 smoke tests passing
- ✅ No regressions in output (table, plaintext, JSON)
- ✅ Build succeeds, no compilation errors
- ✅ Each migrated command tested individually

## Benefits

**Immediate (Phase 1):**
- Type safety for read operations
- Compile-time error detection
- Better IDE support
- ~200 lines of adapter code unused

**After Phase 2:**
- Type safety for write operations
- Full compile-time validation

**After Phase 3:**
- Delete 600 lines of adapter code
- Delete unused legacy types
- Single source of truth (generated types)
- Maintenance burden eliminated

## Risks and Mitigations

**Risk:** Verbose generated type names reduce readability
**Mitigation:** Use short variable names (`f := node.IssueListFields`), helper functions for common patterns

**Risk:** Nil pointer panics from nullable fields
**Mitigation:** Explicit nil checks, sensible defaults, test coverage

**Risk:** Commands break during migration
**Mitigation:** Incremental approach, test after each command, smoke tests catch regressions

**Risk:** Output format changes unintentionally
**Mitigation:** JSON output unchanged (serialization automatic), table output tested visually and via smoke tests

## Future Phases

### Phase 2: Write Operations

**Scope:** `create`, `update`, `delete` commands

**Challenge:** Map inputs → typed `IssueCreateInput` structs
```go
// Current
input := map[string]interface{}{
    "title": title,
    "teamId": teamID,
    "assigneeId": assigneeID,
}

// New
input := IssueCreateInput{
    Title: title,
    TeamId: teamID,
    AssigneeId: &assigneeID, // nullable fields
}
```

**Pattern:** Required fields as values, optional fields as pointers.

### Phase 3: Cleanup

**Delete:**
- `pkg/api/adapter.go` (~1600 lines)
- Unused types from `pkg/api/types.go`
- Conversion helpers

**Update:**
- `CLAUDE.md` - Document direct genqlient usage
- `README.md` - Update architecture section

**Result:** Codebase uses generated types exclusively.

## Implementation Next Steps

1. Write detailed implementation plan (using superpowers:writing-plans)
2. Create worktree for isolation (using superpowers:using-git-worktrees)
3. Execute Phase 1 task-by-task with reviews
4. Document completion, merge to main
5. Plan Phase 2 when ready

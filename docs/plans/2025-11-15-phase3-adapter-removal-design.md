# Phase 3: Complete Adapter Layer Removal - Design Document

**Date:** 2025-11-15
**Status:** Approved
**Prerequisites:** Phase 2 complete (all write operations migrated)

---

## 1. Overview and Goals

**Objective:** Remove the entire adapter layer (2,103 lines) to complete the genqlient migration and achieve the cleanest possible architecture.

**Scope:**
- Delete `pkg/api/adapter.go` (1,674 lines) - all 13 deprecated adapter methods and conversion functions
- Delete `pkg/api/types.go` (429 lines) - all legacy type definitions
- Migrate 2 remaining adapter usages to direct genqlient types
- Comprehensive verification before and after deletion
- Full testing including manual verification of migrated operations

**Success Criteria:**
- Zero adapter method calls remaining in codebase
- Zero legacy type references remaining
- All 39 smoke tests passing
- 2 migrated user lookups tested manually
- Key write operations verified manually (issue create/update, comment create)
- Build succeeds with no compilation errors
- No regressions in functionality

**Benefits:**
- 35% codebase reduction (2,103 lines removed from pkg/api)
- Simplified architecture - no translation layer between commands and API
- All code paths use generated types directly
- Eliminates maintenance burden of conversion functions
- Compile-time type safety throughout

**Non-Goals:**
- Adding new features or commands
- Refactoring command implementations beyond adapter removal
- Performance optimizations (already achieved in Phase 2)

---

## 2. Migration of Remaining Adapter Usages

**Two locations need migration:**

### Migration 1: Issue Update Assignee by Email (`cmd/issue.go:1138`)

**Current code (uses adapter):**
```go
// Look up user by email - still uses adapter (not migrated in Phase 2)
users, err := client.GetUsers(context.Background(), 100, "", "")
if err != nil {
    output.Error(fmt.Sprintf("Failed to get users: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

var foundUser *api.User
for _, user := range users.Nodes {
    if user.Email == assignee || user.Name == assignee {
        foundUser = &user
        break
    }
}

if foundUser == nil {
    output.Error(fmt.Sprintf("User not found: %s", assignee), plaintext, jsonOut)
    os.Exit(1)
}

input.AssigneeId = &foundUser.ID
```

**New code (direct types):**
```go
// Look up user by email or name using generated function
filter := &api.UserFilter{
    Or: []*api.UserFilter{
        {Email: &api.StringComparator{Eq: &assignee}},
        {Name: &api.StringComparator{Eq: &assignee}},
    },
}

userResp, err := api.GetUserByEmail(context.Background(), client, filter)
if err != nil {
    output.Error(fmt.Sprintf("Failed to find user: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

if len(userResp.Users.Nodes) == 0 {
    output.Error(fmt.Sprintf("User not found: %s", assignee), plaintext, jsonOut)
    os.Exit(1)
}

userID := userResp.Users.Nodes[0].UserDetailFields.Id
input.AssigneeId = &userID
```

### Migration 2: User Get Command (`cmd/user.go:187`)

**Current code (uses adapter):**
```go
user, err := client.GetUser(context.Background(), email)
```

**New code (direct types):**
```go
filter := &api.UserFilter{
    Email: &api.StringComparator{Eq: &email},
}

userResp, err := api.GetUserByEmail(context.Background(), client, filter)
if err != nil {
    output.Error(fmt.Sprintf("Failed to get user: %v", err), plaintext, jsonOut)
    os.Exit(1)
}

if len(userResp.Users.Nodes) == 0 {
    output.Error(fmt.Sprintf("User not found with email: %s", email), plaintext, jsonOut)
    os.Exit(1)
}

user := &userResp.Users.Nodes[0].UserDetailFields
```

**Note:** The issue update migration supports lookup by both email AND name (using OR filter), maintaining current functionality.

---

## 3. Pre-Deletion Verification Strategy

### Step 1: Search for Adapter Method Calls

Grep for all 13 deprecated adapter methods across entire codebase:
```bash
# Read operation adapters (10 methods)
grep -r "client\.GetIssues\(" --include="*.go" .
grep -r "client\.GetIssue\(" --include="*.go" .
grep -r "client\.IssueSearch\(" --include="*.go" .
grep -r "client\.GetProjects\(" --include="*.go" .
grep -r "client\.GetProject\(" --include="*.go" .
grep -r "client\.GetTeams\(" --include="*.go" .
grep -r "client\.GetTeam\(" --include="*.go" .
grep -r "client\.GetUsers\(" --include="*.go" .
grep -r "client\.GetViewer\(" --include="*.go" .
grep -r "client\.GetIssueComments\(" --include="*.go" .
grep -r "client\.GetTeamMembers\(" --include="*.go" .

# Write operation adapters (3 methods)
grep -r "client\.CreateIssue\(" --include="*.go" .
grep -r "client\.UpdateIssue\(" --include="*.go" .
grep -r "client\.CreateComment\(" --include="*.go" .
```

**Expected result:** Only 2 matches (GetUsers in issue.go, GetUser in user.go) before migration, 0 after migration.

### Step 2: Search for Legacy Type References

Check for usage of legacy types that would break after deletion:
```bash
# Legacy type references (should all be in adapter.go and types.go)
grep -r "api\.User\b" --include="*.go" . | grep -v "api\.User.*Fields"
grep -r "api\.Issue\b" --include="*.go" . | grep -v "api\.Issue.*Fields"
grep -r "api\.Team\b" --include="*.go" . | grep -v "api\.Team.*Fields"
grep -r "api\.Project\b" --include="*.go" . | grep -v "api\.Project.*Fields"
grep -r "api\.Comment\b" --include="*.go" . | grep -v "api\.Comment.*Fields"
```

**Note:** We exclude matches with "Fields" suffix (e.g., UserDetailFields) as those are generated types we want to keep.

### Step 3: Verify Generated Types Are Sufficient

Confirm that all commands use generated types with fragment fields:
```bash
# Should find usage of generated fragments
grep -r "IssueListFields\|IssueDetailFields" cmd/
grep -r "UserListFields\|UserDetailFields" cmd/
grep -r "TeamListFields\|TeamDetailFields" cmd/
grep -r "ProjectListFields\|ProjectDetailFields" cmd/
grep -r "CommentFields" cmd/
```

**Expected result:** All commands use fragment-based field access (no legacy types).

### Step 4: Create Verification Checklist

Document all verification steps in a checklist format for tracking during implementation.

---

## 4. Testing Strategy

### Phase 3A: Pre-Migration Baseline

Before making any changes:
```bash
# 1. Run smoke tests to establish baseline
make test

# 2. Manually test the 2 operations that will be migrated
./lincli issue update SD-XX --assignee user@example.com  # Test email lookup
./lincli user get user@example.com                        # Test user get

# 3. Record results as baseline
```

### Phase 3B: Post-Migration Testing

After migrating the 2 user lookups but before deleting adapter files:
```bash
# 1. Build to verify compilation
make build

# 2. Run smoke tests (should still pass - 39/39)
make test

# 3. Test migrated user lookup operations
./lincli issue update SD-XX --assignee user@example.com    # By email
./lincli issue update SD-XX --assignee "User Name"         # By name
./lincli issue update SD-XX --assignee invalid@test.com    # Error case
./lincli user get user@example.com                          # User get
./lincli user get invalid@test.com                          # Error case

# 4. Test write operations (Phase 2 functionality)
./lincli issue create --title "Test" --team SD
./lincli issue update SD-XX --title "Updated"
./lincli comment create SD-XX --body "Test comment"
```

### Phase 3C: Post-Deletion Verification

After deleting adapter.go and types.go:
```bash
# 1. Build to verify no compilation errors
make build

# 2. Run full smoke test suite
make test  # Expect 39/39 passing

# 3. Spot check all operation types
# Read operations
./lincli issue list
./lincli project list
./lincli team list
./lincli user list

# Write operations
./lincli issue create --title "Phase 3 verification" --team SD
./lincli issue update SD-XX --state "Done"
./lincli comment create SD-XX --body "Phase 3 complete"

# User lookups (migrated in Phase 3)
./lincli issue update SD-XX --assignee user@example.com
./lincli user get user@example.com
```

### Test Documentation

Create `docs/verification/phase3-testing-results.md` documenting:
- All test commands executed
- Expected vs actual results
- Any errors encountered and resolutions
- Final verification status

### Success Criteria
- Build passes with no errors
- 39/39 smoke tests passing
- All 10 manual test cases passing
- No functional regressions

---

## 5. Implementation Plan and Task Breakdown

### Task 1: Pre-Migration Verification (Est: 15 min)
1. Run baseline smoke tests (`make test`)
2. Manually test user lookup operations (2 test cases)
3. Execute all verification greps (adapter calls, legacy types)
4. Document baseline results
5. Create verification checklist

### Task 2: Migrate Issue Update Assignee Lookup (Est: 20 min)
1. Locate code in `cmd/issue.go:1138`
2. Replace `client.GetUsers()` with `api.GetUserByEmail()` using OR filter
3. Update field access from `foundUser.ID` to `userResp.Users.Nodes[0].UserDetailFields.Id`
4. Update error handling
5. Build and verify compilation
6. Test manually (email lookup, name lookup, error cases)
7. Commit: "Migrate issue update assignee lookup to direct types"

### Task 3: Migrate User Get Command (Est: 10 min)
1. Locate code in `cmd/user.go:187`
2. Replace `client.GetUser()` with `api.GetUserByEmail()` using email filter
3. Update field access to use `UserDetailFields`
4. Update error handling
5. Build and verify compilation
6. Test manually (valid email, invalid email)
7. Commit: "Migrate user get command to direct types"

### Task 4: Post-Migration Verification (Est: 15 min)
1. Run smoke tests (`make test`) - expect 39/39 passing
2. Test all migrated user lookups (5 test cases)
3. Test write operations (3 test cases)
4. Verify no adapter calls remain (`grep` verification)
5. Document test results

### Task 5: Delete Adapter Layer (Est: 10 min)
1. Delete `pkg/api/adapter.go` (1,674 lines)
2. Delete `pkg/api/types.go` (429 lines)
3. Build to verify no compilation errors
4. Fix any import errors if they occur
5. Commit: "Delete adapter layer and legacy types (Phase 3 complete)"

### Task 6: Post-Deletion Verification (Est: 20 min)
1. Run smoke tests - expect 39/39 passing
2. Execute all 10 manual test cases
3. Verify build on clean workspace
4. Document all test results in `docs/verification/phase3-testing-results.md`

### Task 7: Update Documentation (Est: 15 min)
1. Update CLAUDE.md migration status to "Phase 3 Complete"
2. Update file counts and stats
3. Remove references to adapter layer
4. Document final architecture
5. Commit: "Update documentation for Phase 3 completion"

### Task 8: Create Completion Report (Est: 20 min)
1. Create `docs/verification/phase3-completion-report.md`
2. Document all deletions and migrations
3. Include all test results
4. Add before/after metrics
5. Document final architecture state
6. Commit: "Add Phase 3 completion report"

**Total Estimated Time:** 2 hours

### Risk Mitigation
- Incremental commits allow rollback at any step
- Comprehensive verification before deletion
- All tests run before and after deletion
- Manual testing covers migrated operations

---

## 6. Files to be Modified

### Modified
- `cmd/issue.go` - Migrate GetUsers call to GetUserByEmail
- `cmd/user.go` - Migrate GetUser call to GetUserByEmail
- `CLAUDE.md` - Update migration status to Phase 3 complete

### Deleted
- `pkg/api/adapter.go` - 1,674 lines (all adapter methods and conversion functions)
- `pkg/api/types.go` - 429 lines (all legacy type definitions)

### Created
- `docs/verification/phase3-testing-results.md` - Test results documentation
- `docs/verification/phase3-completion-report.md` - Completion report

---

## 7. Expected Outcomes

### Code Metrics
- **Lines deleted:** 2,103 (adapter.go + types.go)
- **Lines added:** ~40 (2 migrations)
- **Net reduction:** ~2,063 lines (35% of pkg/api)
- **Files deleted:** 2
- **Adapter methods removed:** 13
- **Legacy types removed:** All

### Architecture Improvements
- No translation layer between commands and API
- All code paths use generated types directly
- Compile-time type safety throughout
- Simplified package structure in pkg/api

### Maintenance Benefits
- No conversion functions to maintain
- No legacy types to keep in sync
- Single source of truth (generated types)
- Breaking API changes caught at compile time

---

## 8. Rollback Plan

If issues are discovered after deletion:

**Before commit:**
- `git restore pkg/api/adapter.go pkg/api/types.go`
- `git restore cmd/issue.go cmd/user.go`

**After commit:**
- `git revert <commit-hash>` for each problematic commit
- Or `git reset --hard <previous-commit>` if not pushed

**Safety:**
- All changes in git worktree (isolated from main)
- Incremental commits allow selective rollback
- Smoke tests verify each step

---

## Appendix: Verification Checklist

**Pre-Migration:**
- [ ] Baseline smoke tests passing (39/39)
- [ ] Manual test: issue update assignee by email
- [ ] Manual test: user get by email
- [ ] Grep: Only 2 adapter calls found (GetUsers, GetUser)
- [ ] Grep: No legacy type usage outside adapter.go/types.go
- [ ] Grep: All commands use fragment fields

**Post-Migration (before deletion):**
- [ ] Build passes
- [ ] Smoke tests passing (39/39)
- [ ] Manual test: issue update assignee by email
- [ ] Manual test: issue update assignee by name
- [ ] Manual test: issue update assignee - invalid user
- [ ] Manual test: user get by email
- [ ] Manual test: user get - invalid email
- [ ] Manual test: issue create
- [ ] Manual test: issue update
- [ ] Manual test: comment create
- [ ] Grep: Zero adapter calls remaining

**Post-Deletion:**
- [ ] Build passes with no errors
- [ ] Smoke tests passing (39/39)
- [ ] Spot check: issue list
- [ ] Spot check: project list
- [ ] Spot check: team list
- [ ] Spot check: user list
- [ ] Spot check: issue create
- [ ] Spot check: issue update with state
- [ ] Spot check: comment create
- [ ] Spot check: issue update assignee by email
- [ ] Spot check: user get by email
- [ ] Documentation updated
- [ ] Completion report created

---

**Design Approved:** 2025-11-15
**Ready for Implementation:** Yes

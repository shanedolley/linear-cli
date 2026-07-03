# Implementation Plan: Issue Linking

## Task List

### 1. Add GraphQL Operations
**File**: `pkg/api/operations/issues.graphql`

Add mutations:
```graphql
mutation CreateIssueRelation($input: IssueRelationCreateInput!) {
  issueRelationCreate(input: $input) {
    issueRelation {
      id
      type
      issue { identifier title }
      relatedIssue { identifier title }
    }
    success
  }
}

mutation DeleteIssueRelation($id: String!) {
  issueRelationDelete(id: $id) {
    success
  }
}
```

Then run: `go generate ./pkg/api`

### 2. Add Link Command Structure
**File**: `cmd/issue.go`

Add:
- `issueLinkCmd` cobra command
- Flags: `--type` (required), `--remove` (optional)
- Positional args: source issue, target issue
- Validation: self-referential check, type validation

### 3. Implement Relation Types
**File**: `cmd/issue.go`

Implement type handling:
- `blocks`: call `CreateIssueRelation` with type=blocks
- `blocked-by`: swap source/target, call with type=blocks
- `related`: call with type=related
- `duplicate`: call with type=duplicate

### 4. Implement Parent/Child Types
**File**: `cmd/issue.go`

Implement parent/child:
- `parent-of`: get target issue ID, update target's parentId to source
- `sub-issue-of`: get source issue ID, update source's parentId to target

### 5. Implement Remove Functionality
**File**: `cmd/issue.go`

For `--remove`:
- Query source issue's relations
- Find relation matching target and type
- Call `DeleteIssueRelation` with relation ID
- For parent types: set parentId to nil

### 6. Add Smoke Tests
**File**: `smoke_test.sh`

Add tests:
- `test_issue_link_blocks`
- `test_issue_link_blocked_by`
- `test_issue_link_related`
- `test_issue_link_parent_of`
- `test_issue_link_json_output`

## Verification

After each task:
```bash
make build        # Must succeed
make test         # All tests pass
```

Final verification:
```bash
# Manual tests
lincli issue link TEST-1 TEST-2 --type blocks
lincli issue get TEST-1 | grep -i relation
lincli issue link TEST-1 TEST-2 --type blocks --remove
```

## Estimated Scope

- GraphQL: ~20 lines
- Command code: ~200 lines
- Tests: ~30 lines
- Total: ~250 lines of code

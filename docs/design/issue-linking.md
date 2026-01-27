# Design Doc: Issue Linking Command

**Status**: Draft
**Author**: Shane Dolley
**Date**: 2026-01-27

## Summary

Add `lincli issue link` command to create relationships between issues, supporting all Linear relationship types: parent/sub-issue, related, blocked/blocking, and duplicate.

## Motivation

Linear-cli currently cannot create issue relationships, requiring users to fall back to direct GraphQL API calls. This feature enables complete issue management from the CLI.

## Design

### Command Interface

```bash
# Create relation: A blocks B (A must complete before B can start)
lincli issue link ISSUE-A ISSUE-B --type blocks

# Create relation: A is blocked by B
lincli issue link ISSUE-A ISSUE-B --type blocked-by

# Create relation: issues are related
lincli issue link ISSUE-A ISSUE-B --type related

# Create relation: A is duplicate of B
lincli issue link ISSUE-A ISSUE-B --type duplicate

# Create parent/child: A becomes parent of B (B becomes sub-issue of A)
lincli issue link ISSUE-A ISSUE-B --type parent-of

# Create parent/child: A becomes sub-issue of B
lincli issue link ISSUE-A ISSUE-B --type sub-issue-of

# Remove a relation (requires querying existing relations first)
lincli issue link ISSUE-A ISSUE-B --type blocks --remove

# Remove parent relationship (clear parentId)
lincli issue link ISSUE-A --type sub-issue-of --remove
```

**Note on `--remove`**: Deleting relations requires a two-step process:
1. Query issue A's relations to find the relation ID matching B and the type
2. Call `issueRelationDelete(id: relationId)`

### Relation Types Mapping

| CLI Type | Linear API | Direction | Notes |
|----------|-----------|-----------|-------|
| `blocks` | `IssueRelationType.blocks` | issueId=A, relatedIssueId=B | A blocks B |
| `blocked-by` | `IssueRelationType.blocks` | issueId=B, relatedIssueId=A | **Swap internally** - creates "B blocks A" |
| `related` | `IssueRelationType.related` | issueId=A, relatedIssueId=B | Bidirectional |
| `duplicate` | `IssueRelationType.duplicate` | issueId=A, relatedIssueId=B | A is duplicate of B |
| `parent-of` | `IssueUpdateInput.parentId` | Update B's parentId to A's id | **Different API** |
| `sub-issue-of` | `IssueUpdateInput.parentId` | Update A's parentId to B's id | **Different API** |

**Note**: `blocked-by` is not a native Linear relation type. The CLI swaps source/target internally to create a `blocks` relation in the correct direction.

### API Operations

Two new GraphQL operations in `pkg/api/operations/issues.graphql`:

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

### Implementation Steps

1. Add GraphQL mutations to `pkg/api/operations/issues.graphql`
2. Run `go generate ./pkg/api` to generate types
3. Add `issueLinkCmd` to `cmd/issue.go`:
   - Parse source and target issue identifiers
   - Validate `--type` flag
   - For parent/sub-issue types: use `UpdateIssue` with `parentId`
   - For other types: use `CreateIssueRelation`
   - For `--remove`: find existing relation and delete
4. Register command in `init()`
5. Add smoke tests

### Output Format

**Success (default)**:
```
✓ Linked TEAM-123 blocks TEAM-456
```

**Success (--json)**:
```json
{
  "success": true,
  "type": "blocks",
  "issue": "TEAM-123",
  "relatedIssue": "TEAM-456"
}
```

**Success (--plaintext)**:
```
Linked TEAM-123 blocks TEAM-456
```

### Error Handling

| Error | Detection | User Message |
|-------|-----------|--------------|
| Invalid issue identifier | Regex validation | "Invalid issue identifier format: X" |
| Issue not found | API 404 | "Issue not found: TEAM-123" |
| Relation already exists | API error | "Relation already exists between TEAM-123 and TEAM-456" |
| Invalid relation type | Flag validation | "Invalid type 'X'. Valid types: blocks, blocked-by, related, duplicate, parent-of, sub-issue-of" |
| Self-referential | Pre-flight check | "Cannot link an issue to itself" |
| Issue already has parent | API error (for parent-of) | "TEAM-456 already has a parent issue" |

### Validation Rules

1. **Self-referential check**: `if sourceID == targetID { error }`
2. **Type validation**: Must be one of the 6 supported types
3. **Identifier format**: Must match `TEAM-123` pattern (resolved by API)

## Testing

Add to `smoke_test.sh`:
```bash
# Issue link tests
test_issue_link_blocks
test_issue_link_related
test_issue_link_parent
test_issue_link_remove
```

## Verification Checklist

- [ ] `make build` succeeds
- [ ] `make test` passes (39+ tests)
- [ ] Manual test: create blocks relation
- [ ] Manual test: create parent/child relation
- [ ] Manual test: verify relation shows in `issue get`
- [ ] JSON output works for automation

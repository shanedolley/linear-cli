# GraphQL Code Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace 1515 lines of hand-written GraphQL types with auto-generated code using genqlient, improving maintainability as Linear's API evolves.

**Architecture:** Incremental migration keeping existing CLI structure. Add genqlient for code generation from GraphQL operations. Migrate one entity type at a time (Issues → Projects → Teams → Users → Comments). Each phase independently testable.

**Tech Stack:** Go 1.23+, genqlient (GraphQL code generator), Linear GraphQL API

**Working Directory:** `.worktrees/graphql-codegen`

---

## Phase 1: Setup and Configuration

### Task 1: Add genqlient Dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add genqlient package**

Run: `go get github.com/Khan/genqlient@latest`

Expected: Downloads genqlient and updates go.mod

**Step 2: Verify installation**

Run: `go list -m github.com/Khan/genqlient`

Expected: Shows genqlient version (e.g., `github.com/Khan/genqlient v0.7.0`)

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat: add genqlient dependency for GraphQL code generation"
```

---

### Task 2: Download Linear GraphQL Schema

**Files:**
- Create: `pkg/api/schema.graphql`

**Step 1: Create script to download schema**

Create file `scripts/update-schema.sh`:

```bash
#!/bin/bash
# Download Linear's GraphQL schema via introspection

SCHEMA_FILE="pkg/api/schema.graphql"

echo "Downloading Linear GraphQL schema..."

# Get API key from auth file
if [ -f ~/.lincli-auth.json ]; then
    API_KEY=$(jq -r '.api_key' ~/.lincli-auth.json)
else
    echo "Error: No auth file found. Run 'lincli auth' first."
    exit 1
fi

# Download schema using introspection query
curl -X POST https://api.linear.app/graphql \
  -H "Content-Type: application/json" \
  -H "Authorization: $API_KEY" \
  -d '{"query": "query IntrospectionQuery { __schema { queryType { name } mutationType { name } subscriptionType { name } types { ...FullType } directives { name description locations args { ...InputValue } } } } fragment FullType on __Type { kind name description fields(includeDeprecated: true) { name description args { ...InputValue } type { ...TypeRef } isDeprecated deprecationReason } inputFields { ...InputValue } interfaces { ...TypeRef } enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason } possibleTypes { ...TypeRef } } fragment InputValue on __InputValue { name description type { ...TypeRef } defaultValue } fragment TypeRef on __Type { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } } }"}' \
  | jq -r '.data' | graphql-schema-dump > "$SCHEMA_FILE"

# Alternative: Use get-graphql-schema tool
npx -y get-graphql-schema https://api.linear.app/graphql -h "Authorization=$API_KEY" > "$SCHEMA_FILE"

echo "Schema saved to $SCHEMA_FILE"
```

**Step 2: Make script executable and run it**

Run:
```bash
chmod +x scripts/update-schema.sh
./scripts/update-schema.sh
```

Expected: Creates `pkg/api/schema.graphql` with Linear's schema

**Alternative if script fails:**

Manually download using npx:
```bash
mkdir -p scripts
# Get auth token
API_KEY=$(jq -r '.api_key' ~/.lincli-auth.json)
# Download schema
npx -y get-graphql-schema https://api.linear.app/graphql -h "Authorization=$API_KEY" > pkg/api/schema.graphql
```

**Step 3: Verify schema file**

Run: `head -20 pkg/api/schema.graphql`

Expected: Shows GraphQL schema definitions starting with types

**Step 4: Commit**

```bash
git add pkg/api/schema.graphql scripts/update-schema.sh
git commit -m "feat: add Linear GraphQL schema and update script"
```

---

### Task 3: Create genqlient Configuration

**Files:**
- Create: `genqlient.yaml`

**Step 1: Create config file**

Create `genqlient.yaml` in project root:

```yaml
# genqlient configuration for Linear GraphQL code generation
schema: pkg/api/schema.graphql
operations:
  - pkg/api/operations/*.graphql
generated: pkg/api/generated.go
package: api
bindings:
  # Use custom types for time fields
  DateTime:
    type: time.Time
  # Use string for JSON fields
  JSON:
    type: string
  JSONObject:
    type: string
optional: pointer
```

**Step 2: Commit**

```bash
git add genqlient.yaml
git commit -m "feat: add genqlient configuration"
```

---

### Task 4: Create Operations Directory Structure

**Files:**
- Create: `pkg/api/operations/.gitkeep`

**Step 1: Create operations directory**

Run: `mkdir -p pkg/api/operations`

**Step 2: Add gitkeep**

Run: `touch pkg/api/operations/.gitkeep`

**Step 3: Update .gitignore for generated code**

Add to `.gitignore`:
```
# Generated GraphQL code
pkg/api/generated.go
```

**Step 4: Commit**

```bash
git add pkg/api/operations/.gitkeep .gitignore
git commit -m "feat: create operations directory and ignore generated code"
```

---

## Phase 2: Migrate Issues (First Entity)

### Task 5: Create Issues GraphQL Operations

**Files:**
- Create: `pkg/api/operations/issues.graphql`

**Step 1: Write GetIssues query**

Create `pkg/api/operations/issues.graphql`:

```graphql
# Get paginated list of issues with filters
query GetIssues($filter: IssueFilter, $first: Int, $after: String, $orderBy: PaginationOrderBy) {
  issues(filter: $filter, first: $first, after: $after, orderBy: $orderBy) {
    nodes {
      id
      identifier
      title
      description
      priority
      estimate
      createdAt
      updatedAt
      dueDate
      state {
        id
        name
        type
        color
      }
      assignee {
        id
        name
        email
        displayName
      }
      team {
        id
        key
        name
      }
      url
      branchName
      number
      priorityLabel
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}

# Search issues using full-text index
query SearchIssues($term: String!, $filter: IssueFilter, $first: Int, $after: String, $orderBy: PaginationOrderBy, $includeArchived: Boolean) {
  issueSearch(query: $term, filter: $filter, first: $first, after: $after, orderBy: $orderBy, includeArchived: $includeArchived) {
    nodes {
      id
      identifier
      title
      description
      priority
      createdAt
      updatedAt
      state {
        id
        name
        type
      }
      assignee {
        id
        name
        email
      }
      team {
        id
        key
        name
      }
      url
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}

# Get single issue by ID
query GetIssue($id: String!) {
  issue(id: $id) {
    id
    identifier
    title
    description
    priority
    priorityLabel
    estimate
    createdAt
    updatedAt
    dueDate
    url
    branchName
    state {
      id
      name
      type
      color
    }
    assignee {
      id
      name
      email
      displayName
    }
    team {
      id
      key
      name
    }
    parent {
      id
      identifier
      title
    }
    children {
      nodes {
        id
        identifier
        title
        state {
          name
        }
      }
    }
  }
}

# Create new issue
mutation CreateIssue($title: String!, $teamId: String!, $description: String, $assigneeId: String, $priority: Int) {
  issueCreate(input: {
    title: $title
    teamId: $teamId
    description: $description
    assigneeId: $assigneeId
    priority: $priority
  }) {
    success
    issue {
      id
      identifier
      title
      url
    }
  }
}

# Update existing issue
mutation UpdateIssue($id: String!, $title: String, $description: String, $assigneeId: String, $stateId: String, $priority: Int, $dueDate: String) {
  issueUpdate(id: $id, input: {
    title: $title
    description: $description
    assigneeId: $assigneeId
    stateId: $stateId
    priority: $priority
    dueDate: $dueDate
  }) {
    success
    issue {
      id
      identifier
      title
    }
  }
}
```

**Step 2: Commit**

```bash
git add pkg/api/operations/issues.graphql
git commit -m "feat: add Issues GraphQL operations"
```

---

### Task 6: Generate Code for Issues

**Files:**
- Generate: `pkg/api/generated.go`

**Step 1: Run genqlient**

Run: `go run github.com/Khan/genqlient`

Expected: Creates `pkg/api/generated.go` with type-safe functions

**Step 2: Verify generated code**

Run: `wc -l pkg/api/generated.go`

Expected: Shows significant number of lines (500+)

Run: `grep "^func GetIssues" pkg/api/generated.go`

Expected: Shows generated GetIssues function

**Step 3: Add go:generate directive**

Add to `pkg/api/client.go` at the top (after package statement):

```go
//go:generate go run github.com/Khan/genqlient
```

**Step 4: Test generation works**

Run: `go generate ./pkg/api`

Expected: Regenerates without errors

**Step 5: Commit**

```bash
git add pkg/api/generated.go pkg/api/client.go
git commit -m "feat: generate GraphQL code for Issues"
```

---

### Task 7: Create Adapter Functions for Issues

**Files:**
- Create: `pkg/api/adapter.go`

**Step 1: Create adapter file**

Create `pkg/api/adapter.go`:

```go
package api

import (
	"context"
)

// Adapter functions to maintain compatibility with existing cmd/*.go files
// These wrap generated genqlient functions with existing function signatures

// GetIssues wraps the generated GetIssues query
func (c *Client) GetIssues(ctx context.Context, filter map[string]interface{}, limit int, cursor string, orderBy string) (*Issues, error) {
	// Convert filter map to IssueFilter
	var issueFilter *IssueFilter
	if len(filter) > 0 {
		// This will need custom conversion logic based on filter keys
		issueFilter = convertToIssueFilter(filter)
	}

	// Convert orderBy string to enum
	var order *PaginationOrderBy
	if orderBy != "" {
		o := PaginationOrderBy(orderBy)
		order = &o
	}

	// Call generated function
	resp, err := GetIssues(ctx, c, issueFilter, &limit, &cursor, order)
	if err != nil {
		return nil, err
	}

	// Convert response to old Issues type
	return convertToOldIssues(resp.Issues), nil
}

// Helper to convert map filter to IssueFilter type
func convertToIssueFilter(filter map[string]interface{}) *IssueFilter {
	// TODO: Implement based on filter keys
	// For now, return empty filter
	return &IssueFilter{}
}

// Helper to convert generated type to old type
func convertToOldIssues(gen GetIssuesIssuesIssueConnection) *Issues {
	nodes := make([]Issue, len(gen.Nodes))
	for i, n := range gen.Nodes {
		nodes[i] = Issue{
			ID:          n.Id,
			Identifier:  n.Identifier,
			Title:       n.Title,
			Description: n.Description,
			Priority:    n.Priority,
			// Map other fields...
		}
	}

	return &Issues{
		Nodes: nodes,
		PageInfo: PageInfo{
			HasNextPage: gen.PageInfo.HasNextPage,
			EndCursor:   gen.PageInfo.EndCursor,
		},
	}
}
```

**Step 2: Compile to find missing types**

Run: `go build ./pkg/api`

Expected: Compilation errors showing which types need to be defined

**Step 3: Add missing type definitions**

Based on errors, add type definitions to maintain compatibility

**Step 4: Commit**

```bash
git add pkg/api/adapter.go
git commit -m "feat: add adapter functions for Issues migration"
```

---

### Task 8: Test Issues Migration

**Files:**
- Test: existing commands

**Step 1: Run smoke tests**

Run: `make test`

Expected: All 39 tests should pass

**Step 2: If tests fail, debug**

Check which tests fail and fix adapter functions accordingly

**Step 3: Test individual issue commands**

Run:
```bash
./lincli issue list --json | jq '.[0]'
./lincli issue get <issue-id>
```

Expected: Commands work identically to before

**Step 4: Commit any fixes**

```bash
git add <fixed-files>
git commit -m "fix: resolve Issues migration test failures"
```

---

## Phase 3: Migrate Remaining Entities

### Task 9: Migrate Projects

**Files:**
- Create: `pkg/api/operations/projects.graphql`
- Modify: `pkg/api/adapter.go`

**Follow same pattern as Issues:**
1. Create `operations/projects.graphql` with GetProjects, GetProject queries
2. Run `go generate ./pkg/api`
3. Add adapter functions in `adapter.go`
4. Test with `make test`
5. Commit

---

### Task 10: Migrate Teams

**Files:**
- Create: `pkg/api/operations/teams.graphql`
- Modify: `pkg/api/adapter.go`

**Follow same pattern as Issues:**
1. Create operations file
2. Generate code
3. Add adapters
4. Test
5. Commit

---

### Task 11: Migrate Users

**Files:**
- Create: `pkg/api/operations/users.graphql`
- Modify: `pkg/api/adapter.go`

**Follow same pattern**

---

### Task 12: Migrate Comments

**Files:**
- Create: `pkg/api/operations/comments.graphql`
- Modify: `pkg/api/adapter.go`

**Follow same pattern**

---

## Phase 4: Cleanup and Finalization

### Task 13: Delete Old queries.go

**Files:**
- Delete: `pkg/api/queries.go`

**Step 1: Verify all commands still work**

Run: `make test`

Expected: All tests pass without queries.go

**Step 2: Delete old file**

Run: `rm pkg/api/queries.go`

**Step 3: Verify build**

Run: `go build ./...`

Expected: Successful build

**Step 4: Commit**

```bash
git add pkg/api/queries.go
git commit -m "feat: remove hand-written queries.go (1515 lines deleted)"
```

---

### Task 14: Update Documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

**Step 1: Update CLAUDE.md architecture section**

Update the "Architecture" section to document genqlient usage:

```markdown
## Architecture

### GraphQL Code Generation

**Code Generation:** Uses `genqlient` to auto-generate type-safe Go code from GraphQL operations
- **Operations:** `pkg/api/operations/*.graphql` - GraphQL queries and mutations
- **Schema:** `pkg/api/schema.graphql` - Linear's GraphQL schema (updated via script)
- **Generated:** `pkg/api/generated.go` - Auto-generated by genqlient (do not edit)
- **Generation:** Run `go generate ./pkg/api` to regenerate

**Updating Schema:**
When Linear updates their API:
1. Run `./scripts/update-schema.sh` to download new schema
2. Run `go generate ./pkg/api` to regenerate code
3. Compiler will show what changed - fix any breaking changes
4. Test and commit
```

**Step 2: Update README if needed**

Add section about code generation to README development section

**Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: update architecture docs for genqlient migration"
```

---

### Task 15: Final Verification

**Step 1: Clean build**

Run: `make clean && make build`

Expected: Binary builds successfully

**Step 2: Full test suite**

Run: `make test`

Expected: All 39 tests passing

**Step 3: Binary size check**

Run: `ls -lh lincli`

Expected: Binary size still around 13-15MB (similar to before)

**Step 4: Manual testing**

Test key commands:
```bash
./lincli issue list
./lincli issue get <id>
./lincli project list
./lincli team list
```

Expected: All work identically to before migration

**Step 5: Create summary**

Create `docs/graphql-migration-summary.md`:

```markdown
# GraphQL Code Generation Migration Summary

**Date:** 2025-11-14
**Status:** Complete

## Changes

- **Removed:** 1515 lines of hand-written GraphQL types (pkg/api/queries.go)
- **Added:** genqlient code generation
  - 5 operation files (operations/*.graphql)
  - 1 generated file (generated.go)
  - 1 schema file (schema.graphql)
  - 1 adapter layer (adapter.go)

## Benefits

- **Maintainability:** Schema updates are now automated
- **Type Safety:** Compile-time errors for API changes
- **Future-proof:** Easy to add new queries/mutations

## Testing

- All 39 smoke tests passing
- Binary size: ~13MB (unchanged)
- Performance: No regression

## Next Steps

When Linear updates their API:
1. Run `./scripts/update-schema.sh`
2. Run `go generate ./pkg/api`
3. Fix any compilation errors
4. Test and commit
```

**Step 6: Commit summary**

```bash
git add docs/graphql-migration-summary.md
git commit -m "docs: add GraphQL migration summary"
```

---

## Success Criteria

After completing all tasks, verify:

- [ ] All existing commands work identically
- [ ] `make test` shows 39/39 passing
- [ ] Binary size under 15MB
- [ ] `pkg/api/queries.go` deleted (1515 lines removed)
- [ ] Generated code in `pkg/api/generated.go`
- [ ] Operations in `pkg/api/operations/*.graphql`
- [ ] Schema in `pkg/api/schema.graphql`
- [ ] Documentation updated in CLAUDE.md
- [ ] Clean working tree

## Estimated Timeline

- **Task 1-4 (Setup):** 1-2 hours
- **Task 5-8 (Issues):** 3-4 hours
- **Task 9-12 (Other entities):** 4-6 hours
- **Task 13-15 (Cleanup):** 1-2 hours

**Total:** 9-14 hours over 2-3 days

## Notes

- This is an incremental migration - each task is independently committable
- If any task fails, previous commits can be kept and problematic task can be revised
- The adapter layer can be simplified or removed after full migration if generated types match well
- genqlient docs: https://github.com/Khan/genqlient

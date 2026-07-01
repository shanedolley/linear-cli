# CLAUDE.md

Go CLI for Linear's GraphQL API. Personal fork of dorkitude/linctl, renamed to lincli.

## Commands

```bash
make build          # Build binary
make test           # Run smoke tests
make install        # Install to /usr/local/bin (sudo)
make dev-install    # Symlink for development
go generate ./pkg/api  # Regenerate GraphQL code after schema changes
```

## Architecture

```
cmd/           # Cobra commands (issue, project, team, user, comment, attachment)
pkg/api/       # GraphQL client and genqlient-generated code
  operations/  # .graphql files defining queries/mutations
  generated.go # Auto-generated (do not edit)
pkg/auth/      # Auth config (~/.lincli-auth.json)
pkg/output/    # Output formatting (table, plaintext, JSON)
pkg/utils/     # Time expression parser
```

## Key Patterns

**Output modes**: `--json` (agents), `--plaintext` (scripts), default (interactive table)

**Time filtering**: List commands default to 6 months. Use `--newer-than all_time` for all items.

**GraphQL code gen**: Define operations in `pkg/api/operations/*.graphql`, run `go generate ./pkg/api`, use generated functions directly (no adapter layer).

**Authentication**: Personal API key stored in `~/.lincli-auth.json`. Get from https://linear.app/settings/api

## Verification

```bash
./smoke_test.sh      # All 39 tests should pass
lincli auth status   # Verify authentication
lincli issue list --limit 5  # Quick functionality check
```

## AI Agent Notes

- Always use `--json` for programmatic access
- Issue identifiers: "TEAM-123" format
- Team keys: uppercase (e.g., "ENG"), not display names
- User lookups: use email addresses
- Rate limit: 5,000 req/hour

## Adding Commands

See `cmd/CLAUDE.local.md` for command patterns. See `pkg/api/CLAUDE.local.md` for GraphQL patterns.

## Priority Values

0=None, 1=Urgent, 2=High, 3=Normal, 4=Low

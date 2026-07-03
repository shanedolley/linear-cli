# 01 - Discovery: CLI Feature Expansion

## Problem

`lincli` (a fork of dorkitude/linctl) covers only about 15 of the ~506 operations the live
Linear GraphQL API exposes. Missing commands - especially for Initiatives and Projects -
have forced Claude Code and users to call the raw GraphQL API directly. The goal is to close
that gap with first-class commands.

## Desired outcome

Every in-scope domain has native `lincli` commands, so no task requires dropping to the raw
API. Coverage spans Tiers 1-5 of `FEATURE-GAP-ANALYSIS.md`: Initiatives, Projects, Cycles,
Labels, Workflow states, richer Issue and Comment operations, Documents, Teams, Users and
org, Favorites, Views, Notifications, Webhooks, Templates, Releases, Customers/CRM,
Reactions, Triage, Git automation, Time schedules, OAuth apps, Agents, plus the Tier 5
additions (audit log, global search, rate-limit, external links, view preferences,
`userSettingsUpdate`, helper queries).

## Scope

### In scope
- All 147 in-scope checkboxes across Tiers 1-5 of `FEATURE-GAP-ANALYSIS.md` (~330 operations,
  including read/detail/suggestion siblings pulled in with their parent commands).

### Out of scope (bucket B)
- Integration connectors (~56 mutations), auth and session flows, telemetry and onboarding,
  upload internals, push notifications, email intake, org administration, one-time
  issue/project migrations, and deprecated Roadmaps. See the "Excluded from scope" section of
  `FEATURE-GAP-ANALYSIS.md`.

## Key decisions

| Decision | Choice |
|----------|--------|
| Definition of done | Phased by tier; each tier is a working, tested checkpoint |
| Build order | Initiatives, then Projects, then down the tiers |
| Delivery | One PR per tier |
| Output modes | Full parity (table default, `--json`, `--plaintext`) on every new command |
| Destructive ops | Execute immediately, matching existing issue/attachment behaviour (no confirmation prompt) |
| Testing | Read-only smoke tests for new list/get commands; write ops verified manually against a throwaway team/project |

## Success criteria

- Each in-scope operation is reachable through a documented command.
- New read commands are covered by `smoke_test.sh`; the full suite stays green.
- Write commands are manually verified per PR against a non-production team/project.
- Every new command supports table, `--json`, and `--plaintext` output.
- No regression in existing commands.

## Systems touched

- `pkg/api/operations/*.graphql` - new genqlient operations per domain
- `pkg/api/generated.go` - regenerated via `go generate ./pkg/api`
- `cmd/*.go` - new Cobra command groups (initiative, cycle, label, document, etc.)
- `pkg/output/` - formatting for new entity types
- `smoke_test.sh` - new read-command coverage
- `README.md` and `CLAUDE.md` - command documentation

## Constraints and assumptions

- Follow existing patterns: typed genqlient operations (no adapter layer), Cobra command
  structure, `output` package modes, and name-to-ID resolver helpers already used for
  projects and workflow states.
- Keep all 48 existing smoke tests green.
- Schema is already refreshed to the live API; regenerate bindings as operations are added.
- Command naming follows the suggestions in the gap doc (for example `initiative update` for
  editing versus `initiative update-post` for status posts, `oauth-app`, `schedule`).

## Pre-work (before implementation)

- Commit `FEATURE-GAP-ANALYSIS.md` and the brainstorm docs.
- Commit the existing `issue update --project` WIP (`cmd/issue.go`, `pkg/api/client.go`) as
  its own separate commit.
- Handle these at the planning/baseline step (`/brainstorm-04-plan`).

## Open questions

- Which team/project to use as the throwaway target for manual write verification.
- Whether any Tier 4/5 domains (for example Agents API) should be deferred to a later,
  optional phase rather than built in tier order.

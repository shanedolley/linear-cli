# 05 - Code Review Report: CLI Feature Expansion

**Branch:** `feature/cli-feature-expansion` vs `main`
**Complexity:** COMPLEX (12 tasks, 5 tiers)
**Date:** 2026-07-02
**Verdict:** PASS. No critical issues. All important findings fixed and verified.

## Scope

The branch is the full feature expansion: 25,468 lines of hand-written change
across 47 command files, plus regenerated GraphQL bindings. Tiers 1-4 were
reviewed in their own per-task cycles during implementation; task 11 (Tier 5)
also had a code-reviewer pass before this review. This review covers the whole
branch, weighted toward the recent Tier 5 work and cross-cutting concerns.

## Method

Three reviewers ran in parallel:

1. **Plan alignment** - implementation vs `03-design-doc.md` / `04-plan.md`.
2. **Code quality** (`code-reviewer` agent) - correctness, security, error
   handling, resource use.
3. **Simplicity** (`code-simplifier` agent) - over-engineering, dead code,
   duplication.

## Findings

### CRITICAL

None.

### IMPORTANT (all fixed)

| # | Finding | Resolution |
|---|---------|------------|
| 1 | `view prefs create` never validated its parent, though the help said to pass one. | Added an at-least-one-parent check (`--team`/`--project`/`--custom-view`/`--label`). A live probe showed the API rejects zero parents and accepts multiple, so the guard requires at least one rather than exactly one. |
| 2 | The `{}` (empty preferences) limitation was explained only in a test comment. | Documented in `view prefs create` help: the client drops empty objects, so `{}` cannot be sent. |
| 3 | Distinct-kind-flag convention violated: `state create`, `project status`, and `project relate` used a generic `--type`. | Renamed to `--state-type`, `--status-type`, and `--relation`. The old `--type` remains as a hidden alias, so existing scripts keep working. |
| 4 | Dead `Client.Execute` method (~55 lines) duplicated the live `MakeRequest`. | Removed `Execute` and folded its duplicate request struct into `MakeRequest`, leaving one GraphQL request path. |

### SUGGESTION

- Fixed: the `parseJSONObject` doc comment contradicted the actual (reject-empty)
  behavior; rewritten to match.
- Deferred as follow-ups (see backlog): `--custom-view` skips name resolution;
  plaintext output shape differs across commands; `handleRelationLink`
  complexity (pre-existing); dead `stringIn` / `GetRootCmd` (pre-existing);
  resolver duplication in `resolve.go` (intentional house style).

### Dismissed

- "Commit claims 126 smoke tests but the script has ~105" - `126` is the real
  runtime count the suite prints; the reviewer counted `run_test` lines
  statically and missed the loop- and condition-driven tests.

### Noted, no change

- Deferral asymmetry: Triage and SLA shipped read-only, while Releases and
  Agents were fully deferred. Each deferral is recorded and justified in the
  state file's planGate records; the asymmetry is a conscious call, not a gap.
- `link` cannot target documents: the design over-promised; the live API has no
  document parent, which the state file already records.

## Fix-review iterations

**Iteration 1:** applied all four important fixes plus the doc-comment fix,
committed as `110c59a`. Re-verification confirmed every fix builds, passes, and
behaves correctly, and introduced no new issues. No critical or important issues
remained, so the loop exited.

## Verification

- `make build` clean, `go vet ./...` clean, `gofmt -l` clean.
- `go test ./...` passes (cmd + pkg/api).
- `./smoke_test.sh`: 126/126 pass.
- Behavioral checks: renamed flags and their hidden `--type` aliases route
  correctly (state, project status, project relate); `view prefs create` errors
  clearly with no parent and still succeeds with one; write CRUD verified in the
  SD sandbox with no orphaned artifacts.

## Follow-up backlog (non-blocking)

- Add a `resolveCustomView` helper so `view prefs --custom-view` accepts a name.
- Align plaintext output on the Markdown shape (`document`/`initiative` follow
  it; `customer` emits a TSV table).
- Simplify `handleRelationLink` (cyclomatic complexity 33; pre-existing).
- Remove dead `stringIn` (`cmd/issue.go`) and `GetRootCmd` (`cmd/root.go`).
- Decide on a generic name-or-ID resolver if a seventh entity needs one.

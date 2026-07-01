# 04 - Implementation Plan: CLI Feature Expansion

Generated with task-master (models: claude-code / opus for main, research, and fallback).
Source: `03-design-doc.md`.

## Branch and baseline

- **Branch:** `feature/cli-feature-expansion`
- **Baseline (verified before planning):** `make build` clean, `go vet ./...` clean, `make test` 48/48 pass.
- **Pre-work commits on branch:** planning docs; `issue update --project` WIP; CLAUDE.md + gitignore housekeeping.
- **Write-verification sandbox:** `lincli-sandbox` project in the SD team (team creation is plan-capped); team-scoped writes run in SD with logged cleanup.

## Task backlog (12 tasks, phased by PR)

| # | Task | PR | Complexity | Subtasks | Depends on |
|---|------|----|-----------:|---------:|------------|
| 1 | Create shared resolver infrastructure | foundation | 6 | 4 | - |
| 2 | Implement Initiative domain | PR 1 (Tier 1a) | 8 | 6 | 1 |
| 3 | Extend Project domain | PR 2 (Tier 1b) | 7 | 5 | 1, 2 |
| 4 | Cycles, labels, states, issue extensions | PR 3 (Tier 2) | 8 | 6 | 1, 2, 3 |
| 5 | Extend comments, add documents | PR 3 (Tier 2) | 5 | 3 | 4 |
| 6 | Extend teams and users, add org | PR 4 (Tier 3) | 6 | 4 | 5 |
| 7 | Favorites, views, notifications, webhooks, templates | PR 4 (Tier 3) | 7 | 5 | 6 |
| 8 | Releases and Customers/CRM | PR 5a (Tier 4a) | 7 | 4 | 7 |
| 9 | Emoji, triage, git automation, schedules | PR 5b (Tier 4b) | 5 | 4 | 8 |
| 10 | OAuth apps, attachment links, agents | PR 5c (Tier 4c) | 6 | 4 | 9 |
| 11 | Tier 5 cross-cutting operations | PR 6 | 6 | 4 | 10 |
| 12 | Update documentation and final verification | - | 4 | 3 | 11 |

Total recommended subtasks: 52.

## Execution approach

- Build in dependency order. Task 1 (shared resolvers) lands first; it underpins every domain.
- One PR per tier (PR 5 split into 5a/5b/5c), each a working, tested checkpoint.
- Every write command is verified manually against `lincli-sandbox`; new read commands get smoke-test coverage; the 48 existing tests stay green.
- Tier 4 and SLA carry a plan-availability gate: verify the domain is available on the current plan (attempt one scoped write) before building; defer and record if the plan blocks it.

## Files

- Task data: `.taskmaster/tasks/tasks.json`
- Complexity report: `.taskmaster/reports/task-complexity-report.json`
- Resume state: `.taskmaster-state.json`

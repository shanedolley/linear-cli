# Brainstorm: CLI Feature Expansion

**Idea:** Update the lincli (linctl fork) CLI with the features scoped in
`FEATURE-GAP-ANALYSIS.md` - expanding coverage of the live Linear GraphQL API across
Initiatives, Projects, Cycles, Labels, Documents, and the rest of Tiers 1-5.

**External tracking:** None (local brainstorm docs).

**Created:** 2026-07-01

## Progress

| Step | Command | Status |
|------|---------|--------|
| 01 Ideation & Discovery | `/brainstorm-01-ideation` | Complete |
| 02 Complexity | `/brainstorm-02-complexity` | Complete (COMPLEX) |
| 03 Document (Design/PRD) | `/brainstorm-03-document` | Complete (PRD + 7-reviewer pass) |
| 04 Plan | `/brainstorm-04-plan` | Complete (12 tasks / 52 subtasks) |
| 05 Implement | `/brainstorm-05-implement` | Complete (12/12 tasks) |
| 06 Review | `/brainstorm-06-review` | Complete (PASS, 0 critical, 4 important fixed) |
| 07 Verify | `/brainstorm-07-verify` | Pending |
| 08 Finish | `/brainstorm-08-finish` | Pending |

## Key references

- `FEATURE-GAP-ANALYSIS.md` - the scoped backlog (147 in-scope checkboxes across Tiers 1-5)
- `pkg/api/operations/*.graphql` - existing genqlient operation patterns
- `cmd/*.go` - existing Cobra command patterns
- `CLAUDE.md`, `cmd/CLAUDE.local.md`, `pkg/api/CLAUDE.local.md` - conventions

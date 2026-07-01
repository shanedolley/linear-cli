# 02 - Complexity Analysis: CLI Feature Expansion

## Classification: COMPLEX

Every dimension except Dependencies points to COMPLEX, and the overall surface exceeds a
single COMPLEX task - this is a multi-feature program best delivered in tiered phases.

## Six-dimension assessment

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Use Cases | COMPLEX | ~330 operations, 147 checkboxes, ~20 domains. Each operation is well-defined at the API level, but per-command flags and UX still need design. |
| Edge Cases | COMPLEX | Name-to-ID resolution across entity types; nullable fields (the `*string` class just fixed by `derefStr`); pagination; nested-entity rendering (initiative to projects, release to pipeline to stage); destructive ops with no confirmation; deprecated-field avoidance; rate limits. |
| Scope | COMPLEX | Dozens of new `.graphql` operations, regenerated `generated.go`, 10+ new `cmd/*.go` groups, `pkg/output` additions, `smoke_test.sh`, and docs. Touches the whole command layer. |
| Dependencies | SIMPLE | Linear API, genqlient, and Cobra already integrated; schema refreshed; Go toolchain installed; no new libraries. Some cross-domain coupling (initiative to project, issue to label/cycle/milestone). |
| Uncertainty | MODERATE | API-level requirements are clear. Open: per-command flag design, resolver UX, nested output formatting, the throwaway test workspace, and whether the Agents API is deferred. |
| Integration | COMPLEX | Every command threads flags to input building to genqlient to output, with cross-domain references and three output modes each. |

## Why COMPLEX

- Multiple systems and architectural decisions: shared command scaffolding, name-to-ID
  resolver helpers, output rendering for new entity types, and a testing strategy for write
  commands.
- Large, interdependent scope delivered across tiers rather than in one change.
- Requires a design doc that sets conventions once, then applies them per tier.

## Confirmed decisions

- **Complexity:** COMPLEX (confirmed by user).
- **Design-doc scope (Step 03):** all five tiers specified in full detail.

## Structure

The design doc (Step 03) will:

1. Establish shared architecture and conventions once - command grouping, name-to-ID
   resolver helpers, output patterns for all three modes, destructive-op behaviour (execute
   immediately), and the testing strategy.
2. Then specify every command across **all five tiers** in full detail, organised by domain.

Delivery still follows the discovery decision of one PR per tier, built in order
(Initiatives, then Projects, then down the tiers), even though the design covers everything
up front.

## Downstream impact

- Step 03 produces a comprehensive PRD-level design doc (COMPLEX path) covering all tiers,
  with multi-reviewer review.
- Step 04 uses task-master to break the work into tasks, handles the pre-work commits (gap
  doc, `issue --project` WIP) and baseline verification, and creates the feature branch.

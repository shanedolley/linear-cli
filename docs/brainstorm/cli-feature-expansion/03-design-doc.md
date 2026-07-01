# Design Doc / PRD: lincli Feature Expansion

**Status:** Draft
**Author:** Shane Dolley with Claude
**Created:** 2026-07-01
**Last Updated:** 2026-07-02
**Complexity:** COMPLEX
**Source spec:** `FEATURE-GAP-ANALYSIS.md` (Tiers 1-5, in-scope checkboxes only)

---

## 1. Overview

### 1.1 Problem

`lincli` wraps Linear's GraphQL API but covers only about 15 of the roughly 506 operations the live API exposes. The gaps are widest in Initiatives (zero coverage) and Projects (read only). When a command is missing, Claude Code and human users drop to the raw GraphQL API, which defeats the point of a typed, agent-friendly CLI. Each raw call reinvents name-to-ID resolution, output shaping, and error handling that the CLI already solves elsewhere.

### 1.2 Goal

Give every in-scope Linear domain a first-class `lincli` command, so no routine task needs the raw API. Coverage spans all five tiers of `FEATURE-GAP-ANALYSIS.md`: Initiatives, Projects, Cycles, Labels, Workflow states, richer Issue and Comment operations, Documents, Teams, Users and organization, Favorites, Views, Notifications, Webhooks, Templates, Releases, Customers/CRM, Reactions and emoji, Triage, Git automation, Time schedules, OAuth apps, Agents, plus the Tier 5 additions (audit log, global search, rate-limit, external links, view preferences, user settings, and the create/update helper queries).

The build proceeds in tier order, starting with Initiatives and then Projects. Each PR is a working, tested checkpoint. Tier 4 splits across three smaller PRs (see Section 6).

### 1.3 In-scope goals

- Cover every operation marked `[x]` across Tiers 1-5 of the gap analysis (~330 operations including the read, detail, and suggestion siblings pulled in with their parents).
- Match existing conventions: typed genqlient operations, no adapter layer, Cobra command groups, the `pkg/output` three-mode renderer, and name-to-ID resolver helpers.
- Ship full output parity (table default, `--json`, `--plaintext`) on every new command.
- Keep the 48 existing smoke tests green and add read-only smoke tests for new list and get commands.

### 1.4 Non-goals (bucket B, excluded from scope)

The following stay out, matching the "Excluded from scope" section of the gap analysis. Do not build commands for them.

| Excluded area | Examples | Reason |
|---|---|---|
| Integration connectors (~56 mutations plus read/settings queries) | `integrationSlack*`, `integrationGithub*`, `integrationJira*`, `integrationMicrosoftTeams*`, `integrationGitlab*`, `integrationSalesforce*`, `integrationZendesk`, `integrationIntercom*`, `integrationSentryConnect`, `integrationOpsgenie*`, `integrationPagerDuty*`, `integrationFigma`, `integrationDiscord`, `integrationGong`, `integrationMcpServer*`, `integration(s)`, `integrationTemplate*`, `integrationsSettings*` | OAuth handshakes and connector settings belong in the web app |
| Auth and sessions | `googleUserAccountAuth`, `samlTokenUserAccountAuth`, `passkeyLogin*`, `emailTokenUserAccountAuth`, `logout*`, `userRevoke*Session(s)`, `userSessions`, `authenticationSessions`, `ssoUrlFromEmail` | Handled by `auth` command and browser flows |
| Telemetry, onboarding, exports | `trackAnonymousEvent`, `createCsvExportReport`, `createOrganizationFromOnboarding`, `joinOrganizationFromOnboarding`, `leaveOrganization`, `contactCreate`, `contactSalesCreate` | Not CLI concerns |
| Upload internals | `fileUpload`, `fileUploadDangerouslyDelete`, `imageUploadFromUrl`, `importFileUpload`, `refreshGoogleSheetsData` | Already used internally by `attachment upload` |
| Push notifications and device | `pushSubscription*` | Device-specific |
| Email intake | `emailIntakeAddress*` | Web-app administration |
| Org administration | `organizationDomain*`, `organizationDelete`, `organizationCancelDelete`, `organizationDeleteChallenge`, `organizationStartTrialForPlan`, `userFlagUpdate`, `userSettingsFlagsReset`, `userDiscordConnect`, `userExternalUserDisconnect`, `userUnlinkFromIdentityProvider` | Destructive account operations, web-app only |
| Issue and project migration | `issueImportCreate*`, `issueImportUpdate/Process/Delete/Check*`, `issueExternalSyncDisable`, `issueDescriptionUpdateFromFront`, `projectExternalSyncDisable`, `projectCreateSlackChannel` | One-time bulk imports, done via web importers |
| Roadmaps (deprecated) | `roadmapCreate/Update/Delete`, `roadmapToProject*`, roadmap queries | Deprecated by Linear in favor of Initiatives |

Also note these deprecated write paths inside otherwise in-scope domains, which the design routes around: `projectArchive` (use `projectDelete` or set state via `projectUpdate` `statusId`), `projectUpdateDelete` (use `projectUpdateArchive`), `notificationSubscriptionDelete` (set `active=false`), and `attachmentIssue` as a query (use `attachmentsForURL`).

---

## 2. Success Metrics

| Metric | Baseline | Target | Measurement |
|---|---|---|---|
| In-scope operations reachable via a command | ~15 | ~330 (100% of in-scope) | Manual audit against the gap checklist per tier PR |
| Domains with write coverage | 3 (issue, comment, attachment) | ~20 | Command inventory per PR |
| Read commands under smoke tests | 48 tests | 48 plus new list/get coverage per tier | `./smoke_test.sh` passes at each PR |
| Output-mode parity on new commands | n/a | 100% (table, `--json`, `--plaintext`) | Per-command review checklist |
| Regressions in existing commands | 0 | 0 | Smoke suite stays green |
| Raw-API fallbacks for routine work | Not instrumented (no baseline) | Observed: no in-scope domain forces raw GraphQL | Qualitative; contributors log any fallback they hit in the PR's write-verification notes |

The CLI has no instrumentation that counts raw-API fallbacks, so this metric stays observed and qualitative. Contributors record any fallback they needed in the PR write-verification log (Section 3.8) with the missing operation, which turns anecdotes into a lightweight signal for follow-up work.

A tier is "done" when every in-scope operation in that tier has a documented command, read commands have smoke tests, write commands are verified against the `lincli-sandbox` project (and the SD team for team-scoped writes, per Section 3.8), all three output modes work, and the full suite passes.

A domain deferred by the plan-availability gate (Section 4.5) is excluded from the "100% of in-scope" denominator and does not block "tier done." Each deferral is recorded explicitly in the PR with the domain and the reason (the plan or permission error observed), so the excluded scope stays visible for a later plan upgrade.

---

## 3. Shared Architecture and Conventions

These rules apply to every command in every tier. Specify once here, apply everywhere.

### 3.1 Command grouping

Each domain gets one Cobra command group in `cmd/`, registered on `rootCmd` in an `init()`. Subcommands attach to the group. Follow the existing shape in `cmd/issue.go` and `cmd/project.go`.

| Pattern | Rule |
|---|---|
| New group file | `cmd/<domain>.go` (for example `cmd/initiative.go`, `cmd/cycle.go`, `cmd/release.go`) |
| Group registration | `rootCmd.AddCommand(<domain>Cmd)` in `init()` |
| Subcommand registration | `<domain>Cmd.AddCommand(<sub>Cmd)` in `init()` |
| Nested groups | Sub-groups attach to their parent group, not root (for example `project milestone`, `project label`, `project status`, `release pipeline`, `release stage`, `release note`, `customer need`, `customer status`, `customer tier`) |
| Verb naming | `list`/`ls`, `get`/`show`, `create`/`new`, `update`, `delete`, `archive`, `unarchive`; domain verbs match the gap doc |
| Standard command body | 1) read output flags via `viper.GetBool("plaintext"/"json")`, 2) `auth.GetAuthHeader()`, 3) `api.NewClient(authHeader)`, 4) build typed filter or input, 5) call the generated function with `context.Background()`, 6) render via the `output` package |

Naming conventions carried from the decisions, honored exactly:

- Three `update*` verbs are defined together so they never get confused: `update` edits the entity (for example `initiative update <id>`), `update-post` posts and manages status updates (`initiative update-post ...`), and `update-reminder` creates a reminder to post the next update (`initiative update-reminder <id>`). The same three apply to projects (`project update`, `project update-post`, `project update-reminder`). Each command's `Long` help text names all three siblings and states which one the user wants.
- `oauth-app` for developer OAuth application management.
- `schedule` for time schedules / on-call.
- `state` for workflow states.
- `label` for issue labels; `project label` for project labels (a separate nested group).

Cross-domain UX rules (apply everywhere):

- **One add-child shape.** Managing a child collection uses the nested pattern `<parent> <child> add <parent-ref> <child-ref>` and `<parent> <child> remove <parent-ref> <child-ref>`. Apply it consistently: `initiative project add/remove`, `initiative label add/remove`, `project label add/remove`. Do not mix in flat verbs such as `add-project` or `add-member` for these collections.
- **Distinct kind flags, not a shared `--type`.** A `--type` flag that means something different in each domain confuses users and agents. Give each domain a specific flag name: `issue relate --relation <kind>`, `attachment link --provider <name>`, `favorite --entity <kind>`, `state create --state-type <kind>`, `project status create --status-type <kind>`. Reserve plain nouns for the flag name that matches the domain concept.
- **`set-role` is qualified in help.** `team ... set-role` sets a team-membership role; `user set-role` sets an org role. Each command's help states which scope it changes.

### 3.2 genqlient operation workflow

Follow `pkg/api/CLAUDE.local.md`. No adapter layer; commands call generated functions and read fragment fields directly.

1. Define operations in `pkg/api/operations/<domain>.graphql`. One file per domain group.
2. Reuse a `ListFields` fragment for tables and a `DetailFields` fragment for single-item views, matching `IssueListFields`/`IssueDetailFields` and `ProjectListFields`/`ProjectDetailFields`.
3. Run `go generate ./pkg/api` to regenerate `pkg/api/generated.go`. Never edit `generated.go` by hand.
4. Use the generated function directly: `resp, err := api.ListInitiatives(ctx, client, filter, &limit, nil, orderByEnum)`.
5. Access data through the fragment: `node.InitiativeListFields.Name`.

Operation-name rules (these become the Go function names, so they must be unique and descriptive):

- Give each `query`/`mutation` a unique Go-friendly name even when the backing Linear operation name collides. The clearest collision is Linear's `initiativeUpdate`: the **mutation** edits the initiative, while the **query** fetches one status update. Name them apart, for example `UpdateInitiative` (mutation `initiativeUpdate`) and `GetInitiativeUpdate` (query `initiativeUpdate`). The same care applies to `projectUpdate` (mutation edits the project; query fetches one status update).
- Prefer verbs that mirror the command: `ListInitiatives`, `GetInitiative`, `CreateInitiative`, `UpdateInitiative`, `ArchiveInitiative`.
- The existing but unwired `commentUpdate` operation in `operations/comments.graphql` gets a command in Tier 2; reuse it rather than redefining.

Input and filter typing:

- Filters use typed structs (`api.InitiativeFilter`, `api.CycleFilter`, and so on). Build them in a `build<Domain>FilterTyped(cmd)` helper, matching `buildProjectFilterTyped`.
- Inputs pass as pointers (`&api.InitiativeCreateInput{}`). Optional fields set only when the flag changed, via `cmd.Flags().Changed("...")`.
- To clear a nullable field, send `api.NullSentinel`; the client's `stripNulls` converts it to JSON `null` (see `project update --project none`).

### 3.3 Name-to-ID resolver strategy

Users and agents pass human-friendly identifiers; the API wants UUIDs. Consolidate the resolution logic that already exists piecemeal (team by key, project by name, user by email, workflow state by name) into shared helpers in a new `cmd/resolve.go`, then reuse it across domains.

The rule is concrete and applies to every entity reference on every command:

- **Every reference accepts either an ID/identifier or a name.** A raw UUID (or issue identifier such as `TEAM-123`, or team key such as `ENG`) returns unchanged. Any other value is treated as a name and resolved.
- **Names resolve case-insensitively,** scoped to the relevant team or parent where the entity is scoped (workflow states and cycles within a team, milestones within a project, and so on).
- **Prefer an exact match.** If exactly one entity matches, use it.
- **On multiple matches, error and list the candidates with their IDs,** then exit non-zero. The message shows each candidate's name and UUID so the caller can rerun with an unambiguous ID.
- **On no match, error and, where the candidate set is small and bounded** (states, statuses, labels within a team), list the valid options, matching the existing `issue update --state` error. Exit non-zero.
- **Recommend IDs for scripts and agents.** Help text and the CLAUDE docs state that names are a convenience for interactive use and that IDs avoid ambiguity and an extra lookup.
- **All resolvers are bounded to one workspace.** Every lookup runs under the configured personal API key, which is tied to a single workspace, so cross-tenant or cross-workspace resolution cannot happen.

| Entity | Accepts | Resolution path | Backing operation |
|---|---|---|---|
| Team | key (uppercase, e.g. `ENG`) or UUID | direct key lookup | `GetTeam` (existing) |
| User | email or name or UUID | `UserFilter{Or: [{Email eq}, {Name eq}]}` | `GetUserByEmail` (existing) |
| Issue | identifier `TEAM-123` or UUID | direct | `GetIssue` (existing) |
| Project | name (case-insensitive) or UUID | `ProjectFilter{Name}` | `ListProjects` (existing) |
| Workflow state | name within a team or UUID | embedded `team.states.nodes`, case-insensitive | `GetTeamStates` embed |
| Initiative | name or UUID | `InitiativeFilter{Name}` | `ListInitiatives` |
| Cycle | number or name within a team, or UUID | `CycleFilter{Team, Number/Name}` | `cycles` |
| Issue label | name within team or workspace, or UUID | `IssueLabelFilter{Name}` | `issueLabels` |
| Project label | name or UUID | `projectLabels` filter | `projectLabels` |
| Milestone | name within a project or UUID | `projectMilestones(filter)` | `projectMilestones` |
| Project status | name or UUID | `projectStatuses` | `projectStatuses` |
| Release / pipeline / stage | name or UUID | domain list query | `releases`, `releasePipelines`, `releaseStages` |
| Customer / status / tier | name or UUID | domain list query | `customers`, `customerStatuses`, `customerTiers` |

Resolver caching:

- **Cache each resolver's lookups within a single command invocation.** A resolver fetches its candidate set once per run and answers repeated lookups from that cache. This matters for `issue batch-create`/`issue batch-update`, which resolve the same team, labels, cycle, or assignee across many rows: without a per-invocation cache, a large batch re-resolves per row and burns the 5,000 req/hour budget.
- Cache nothing across process runs; each new command run resolves fresh.
- Where a parent already embeds the child set (workflow states inside `team.states`), reuse the embed rather than making an extra call, matching the current state-update path.

### 3.4 Output rendering across three modes

Every command supports all three modes with full parity, using `pkg/output`. Mode selection reads `viper.GetBool("plaintext")` and `viper.GetBool("json")`.

| Mode | Flag | Renderer | Shape |
|---|---|---|---|
| Table (default) | none | `output.Table(TableData{Headers, Rows}, plaintext, jsonOut)` | Columns picked from the `ListFields` fragment; truncate long text with `truncateString` |
| JSON | `-j`/`--json` | `output.JSON(nodes)` for lists, `output.JSON(detail)` for single items | Raw fragment structs, so agents get stable field names |
| Plaintext | `-p`/`--plaintext` | Markdown-style headings and bullet lists | `# <Domain>`, `## <Name>`, `- **Field**: value`; ends with `Total: N` |

Rules:

- Lists render a header set derived from the `ListFields` fragment; get commands render the fuller `DetailFields` fragment.
- Guard every nullable pointer field before dereferencing; use `derefStr` for `*string`, matching `issue.go` and `project.go`.
- Empty results call `output.Info("No <items> found", plaintext, jsonOut)` and return, matching `issue list`.
- Write commands confirm with `output.Success(...)` (or a colored line in default mode) and emit the created/updated entity in `--json`.
- Errors always route through `output.Error(msg, plaintext, jsonOut)` then `os.Exit(1)`.

Nested-entity rendering:

- `initiative get` shows the initiative header, then a `## Projects` section listing linked projects (via `initiativeToProjects`), then labels and relations. In JSON, nest the projects under the initiative fragment.
- `release get` shows the release, then its pipeline, then the pipeline's stages, as nested sections (`## Pipeline`, `### Stages`). JSON nests pipeline -> stages.
- `cycle get` shows progress and scope history inline.
- `project get` keeps its existing nested sections (teams, members, updates, documents, issues) and gains milestones, labels, relations, and status.
- **Table-mode default for nested one-to-many sections** (initiative -> projects, release -> pipeline -> stages): print a summary count line, then an indented list or a sub-table of the children. Keep the same field set the child's own `list` uses, so the nested view and the standalone list read alike.

Query-depth and complexity guard:

- **Deep nested `DetailFields` fragments risk hitting Linear's query-depth and complexity limits.** A `get` that inlines projects, labels, relations, milestones, and status in one request can be rejected. When a request is rejected for depth or complexity, fall back to separate, paginated calls per nested section and assemble the view client-side. Prefer the single deep query for speed, but keep the split path ready for the heaviest entities (initiatives and projects).

### 3.5 Pagination, limit, and time-filter flags

Match the existing conventions exactly so behavior stays predictable.

| Flag | Short | Default | Applies to | Notes |
|---|---|---|---|---|
| `--limit` | `-l` | 50 | all list commands | Converted to `*int`; passed as `first` |
| `--sort` | `-o` | `linear` | list commands that support ordering | Maps to `PaginationOrderBy` (`linear` -> nil, `created`, `updated`); invalid value errors out |
| `--newer-than` | `-n` | varies (see below) | time-filterable lists | Parsed by `utils.ParseTimeExpression`; `all_time` disables the filter |
| `--include-completed` | `-c` | false | lists with lifecycle state | Excludes `completed`/`canceled` unless set, matching `project list` |
| `--include-archived` | none | false | lists that support it | Passed as `includeArchived` where the API accepts it |
| `--team` | `-t` | none | scoped lists (cycles, labels, states, templates) | Resolved to team ID |

Default time filter:

- High-churn, high-cardinality lists (issues, comments) keep the existing `6_months_ago` default.
- **Long-lived, low-cardinality entities default to no time filter (`all_time`):** initiatives, documents, releases, customers, and cycles. These lists are small and users expect to see all of them, so a six-month cutoff would silently hide relevant rows.

Pagination:

- Cursor pagination uses `pageInfo { hasNextPage endCursor }` in every list fragment, as today.
- **Surface `pageInfo.hasNextPage` as a "more available" hint.** When a list is truncated by `--limit`, print a trailing hint (for example `More available; rerun with a higher --limit`) in table and plaintext modes, and include `hasNextPage`/`endCursor` in the JSON payload. Results must never look complete when they are not.
- The CLI fetches a single page sized by `--limit`; deeper pagination is out of scope for this expansion, but the fragments keep `pageInfo` so a future `--all` flag can build on it.

### 3.6 Destructive-operation behavior

Destructive commands (`delete`, `archive`, `remove`, `retire`, `unsync`, `merge`, `rotate-secret`) execute immediately with no confirmation prompt, matching the current `issue`/`attachment` behavior. Rules:

- The command performs the mutation directly and reports success via `output.Success` or a colored line.
- `--json` returns `{"success": true}` or the affected entity.
- Prefer non-deprecated paths: archive a project through `projectUpdate` (set `statusId`) or `projectDelete` (not `projectArchive`); archive a project status update through `projectUpdateArchive` (not `projectUpdateDelete`); deactivate a notification subscription through `notificationSubscriptionUpdate` with `active=false` (not `notificationSubscriptionDelete`).
- Secret-rotating commands print the new secret once in the success payload, since Linear returns it only at rotation time.

**Accepted risk (owner-reviewed).** The execute-immediately policy is deliberate and applies with no exceptions, including high-blast-radius and irreversible operations: `team delete`, `customer merge`, `user suspend`, `oauth-app rotate-secret`, `webhook rotate-secret`, and `issue batch-update`. Reviewers flagged that a blanket no-prompt policy now covers these; the owner reviewed and accepted it in exchange for full script and agent friendliness. Two consequences are documented, not fixed:

- **Secret-rotation output.** `oauth-app rotate-secret`, `oauth-app rotate-webhook-secret`, and `webhook rotate-secret` print the new secret to stdout once. The caller is responsible for shell history, scrollback, and log capture; the CLI adds no masking.
- **No client-side batch guard.** `issue batch-create` and `issue batch-update` rely on Linear's server-side limits. The CLI enforces no client-side maximum batch size, so an oversized or wrong-target batch executes as written.

Bulk operations add client-side backoff on rate-limit errors (see Section 3.7) so a large batch retries rather than failing outright.

### 3.7 Error handling

- Authentication failure: `output.Error("Not authenticated. Run 'lincli auth' first.", ...)`, exit 1.
- API failure: wrap with context, `output.Error(fmt.Sprintf("Failed to <verb> <entity>: %v", err), ...)`, exit 1.
- Validation failure (missing required flag, bad enum, self-referential link): validate before calling the API, error with the allowed values, exit 1. Section 3.9 lists the enums whose values appear in these messages.
- Not found: report the identifier that failed and, where the candidate set is small, list the valid options.
- Rate limiting: surface Linear's error verbatim; the `rate-limit` command (Tier 5) lets users inspect the remaining budget.
- **Bulk backoff:** batch and bulk commands (`issue batch-create`, `issue batch-update`) retry on a rate-limit error with client-side exponential backoff before giving up, so a single throttle does not abort a large run.

### 3.8 Testing strategy

| Test type | Scope | Mechanism |
|---|---|---|
| Read smoke tests | new `list`/`get`/`search` commands per tier | Extend `smoke_test.sh` with runs in all three modes, following the existing `run_test` pattern; discover IDs from prior list output |
| Help and validation tests | new command groups | Assert `--help` lists subcommands and that invalid flags/enums error gracefully, matching the `issue link` checks |
| Write verification | every create/update/delete/archive command | Manual, per PR, against the `lincli-sandbox` project for project-scoped writes and the SD team for team-scoped writes; never run destructive automated tests against live data |
| Regression | whole suite | `./smoke_test.sh` stays green (48 existing plus new read tests) at each PR |
| Flag regression | new `issue create`/`update` flags | Prove `--label`, `--cycle`, `--milestone`, `--estimate`, and `--parent` do not break existing issue flags (see below) |
| Build | whole repo | `make build` after every `go generate ./pkg/api` |

Test and sandbox target:

- Team creation is plan-capped on this workspace (Free/Standard tier, 5-team limit reached), so a dedicated throwaway test team is not available.
- The write-verification sandbox is a dedicated project, `lincli-sandbox` (id `39b8fc96-97b5-48da-9050-6d8b9e911dfb`, url https://linear.app/shane-dolley/project/lincli-sandbox-122465ab12ef), in the existing **SD** team. Project-scoped writes (projects, milestones, project status updates, project labels, documents, issues) stay isolated there.
- Team-scoped writes (issue labels, workflow states, cycles) run in the SD team and clean up after themselves, because they cannot be confined to a project.

Write-verification artifact convention:

- Each PR description carries a write-verification log. For every write command, the log records the exact command run, the expected output, and the actual output, all against `lincli-sandbox` (or the SD team for team-scoped writes). This makes destructive, un-prompted operations auditable in review even though no automated test exercises them.
- The new `issue create`/`update` flags (`--label`, `--cycle`, `--milestone`, `--estimate`, `--parent`) include a regression check in the log proving that existing issue flags (project, assignee, state, priority, title, description) still behave as before when the new flags are unset and when they are combined.
- Team-scoped writes in the SD team (issue labels, workflow states, cycles) carry a cleanup entry in the log: it records that each created artifact was deleted or archived after verification, so no orphaned data accumulates in the live team.

Read smoke tests must be safe to run repeatedly against a live workspace: list, get, and search only. Write operations are verified by hand and documented in the PR description.

### 3.9 Validation enum reference

Validation errors list the allowed values. These come straight from the schema; confirm the exact spellings after each `go generate` in case Linear changes them.

| Flag | Backing type | Valid values |
|---|---|---|
| `initiative ... --status` | `InitiativeStatus` | `Proposed`, `Planned`, `Active`, `Completed`, `Canceled` |
| `initiative update-post ... --health` | `InitiativeUpdateHealthType` | `onTrack`, `atRisk`, `offTrack` |
| `initiative set-lead --mode` | `InitiativeLeadTeamChangeMode` | `selectedOnly`, `includeDescendants` |
| `project update-post ... --health` | `ProjectUpdateHealthType` | `onTrack`, `atRisk`, `offTrack` |
| `project status create/update --status-type` | `ProjectStatusType` | `backlog`, `planned`, `started`, `paused`, `completed`, `canceled` |
| `state create/update --state-type` | `WorkflowStateCreateInput.type` | `backlog`, `unstarted`, `started`, `completed`, `canceled` |
| `user set-role --role` | `UserRoleType` | `owner`, `admin`, `guest`, `user`, `app` (`member` is not valid here; it belongs to `TeamRoleType`) |
| `team ... set-role --role` | `TeamRoleType` | `owner`, `member` |

---

## 4. Per-Tier Requirements

Each tier ships as one PR, except Tier 4, which splits into three PRs (see Section 6). Within a tier, each domain lists its commands, key flags, backing GraphQL operations (exact Linear names), output shape, and acceptance criteria.

### 4.1 Tier 1 - Initiatives (PR 1)

Initiatives have zero coverage today; this is the largest single gap and ships first. New group: `cmd/initiative.go`. New operations: `pkg/api/operations/initiatives.graphql`.

The Tier 1 acceptance criteria below are written as concrete, testable statements (exit codes, exact error text, expected output fields). They are the template for every later tier: later tiers state acceptance in the same shape rather than repeating the full detail.

#### 4.1.1 Initiative CRUD

| Command | Key flags / args | Backing operations |
|---|---|---|
| `initiative list` | `--limit`, `--sort`, `--newer-than` (default `all_time`), `--include-archived` | Query `initiatives` |
| `initiative get <id>` | id or name arg | Query `initiative` (with `initiativeToProjects` embedded for the projects section) |
| `initiative create` | `--name` (required), `--description`, `--owner`, `--target-date`, `--status` | Mutation `initiativeCreate` |
| `initiative update <id>` | `--name`, `--description`, `--owner`, `--target-date`, `--status` | Mutation `initiativeUpdate` (Go op name `UpdateInitiative`) |
| `initiative archive <id>` / `initiative unarchive <id>` | id arg | Mutation `initiativeArchive`, `initiativeUnarchive` |
| `initiative delete <id>` | id arg | Mutation `initiativeDelete` |
| `initiative set-lead <id>` | `--team` (lead team, required), `--mode` (`selectedOnly` default, or `includeDescendants`) | Mutation `initiativeLeadTeamUpdate` |

Command-to-operation notes:

- `--owner` sets a person as the initiative owner. It maps to `initiativeUpdate.ownerId`; the CLI resolves the user by email or name. Owner lives on the initiative record, not on the lead-team mutation.
- `--status` maps to `initiativeUpdate.status` (`InitiativeStatus`: `Proposed`, `Planned`, `Active`, `Completed`, `Canceled`).
- `initiative set-lead` sets the lead **team**, not a person. It maps to `initiativeLeadTeamUpdate`, whose only inputs are `id`, `leadTeamId`, and `mode`. `--mode includeDescendants` also updates matching editable sub-initiatives; `--mode selectedOnly` (the default) updates just this initiative. There is no person-lead concept on this mutation; a person is set as owner via `initiative update --owner`.

Output: list table columns Name, Status, Owner, Target Date, Progress, Updated. Get renders the header plus Projects, Labels, and Relations sections.

Acceptance (template for later tiers):

- `initiative list` in default, `--json`, and `--plaintext` exits 0 and prints the six columns above; with no initiatives it prints `No initiatives found` and exits 0.
- `initiative get <name>` resolves the name case-insensitively and prints the header plus the Projects, Labels, and Relations sections; a `--json` run returns one object with those nested arrays.
- `initiative get <unknown>` exits non-zero with `Initiative not found: <unknown>`; when two initiatives share a name it exits non-zero and lists each candidate with its UUID.
- `initiative create --name X` returns the new initiative (id and name printed; full object under `--json`) and exits 0; omitting `--name` exits non-zero with `--name is required`.
- `initiative update <id>` sends only the fields whose flags changed; unchanged fields are absent from the mutation input.
- `initiative create/update --status <bad>` exits non-zero and lists the five valid `InitiativeStatus` values.
- `--owner <email>` resolves to a user id; an unknown email exits non-zero with the not-found message.
- `--target-date` accepts `YYYY-MM-DD`; any other format exits non-zero with the expected format.
- `initiative set-lead <id> --team <key>` exits 0 and reports the new lead team; `--mode <bad>` exits non-zero and lists `selectedOnly` and `includeDescendants`.
- `archive`, `unarchive`, and `delete` execute immediately, print success, and exit 0.

#### 4.1.2 Initiative-to-project links (nested group `initiative project`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `initiative project add <initiative> <project>` | initiative and project args | Mutation `initiativeToProjectCreate` |
| `initiative project remove <initiative> <project>` | args | Mutation `initiativeToProjectDelete` |
| `initiative project reorder <initiative> <project>` | `--sort-order` | Mutation `initiativeToProjectUpdate` |
| (read) linked projects surface inside `initiative get` | | Query `initiativeToProjects`, `initiativeToProject` |

- Acceptance: project resolves by name or ID; `initiative get` lists linked projects with sort order; `reorder` changes position; `remove` executes immediately. `add` returns the new link under `--json`.

#### 4.1.3 Initiative labels and relations

| Command | Key flags / args | Backing operations |
|---|---|---|
| `initiative label add <initiative> <label>` / `initiative label remove <initiative> <label>` | args | Mutation `initiativeAddLabel`, `initiativeRemoveLabel` |
| `initiative relate <id> <other>` | `--relation <kind>`, `--remove` | Mutation `initiativeRelationCreate`, `initiativeRelationUpdate`, `initiativeRelationDelete`; Query `initiativeRelations` |

- `initiative label add/remove` follows the standard nested add-child shape (Section 3.1), matching `project label add/remove`.
- `initiative relate` uses `--relation` (not `--type`) for the relation kind, per the distinct-kind-flag rule.
- Acceptance: label and related initiative resolve by name or ID; `--remove` deletes the relation by looking it up on the source; relations render in `initiative get`.

#### 4.1.4 Initiative status updates (`update-post`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `initiative update-post list <id>` | `--limit` | Query `initiativeUpdates`, `initiativeUpdate` (Go op name `GetInitiativeUpdate`) |
| `initiative update-post create <id>` | `--body` (required), `--health` | Mutation `initiativeUpdateCreate` |
| `initiative update-post edit <post-id>` | `--body`, `--health` | Mutation `initiativeUpdateUpdate` |
| `initiative update-post archive <post-id>` / `unarchive <post-id>` | args | Mutation `initiativeUpdateArchive`, `initiativeUpdateUnarchive` |
| `initiative update-reminder <id>` | id arg | Mutation `createInitiativeUpdateReminder` |

- `--health` maps to `InitiativeUpdateCreateInput.health` (`InitiativeUpdateHealthType`: `onTrack`, `atRisk`, `offTrack`). Health is a property of a status update, not of the initiative record.
- `initiative update-reminder` creates a reminder to post the next initiative update; its help states this, since it backs `createInitiativeUpdateReminder` rather than editing anything.
- Acceptance: the `update` versus `update-post` split is honored so editing the initiative and posting a status update never collide; `--health <bad>` exits non-zero and lists the three health values; list renders body, health, author, date.

### 4.2 Tier 1 - Projects (PR 2)

Projects are read only today. Extend `cmd/project.go`; add nested groups. New/extended operations in `pkg/api/operations/projects.graphql`.

#### 4.2.1 Project CRUD

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project create` | `--name` (required), `--description`, `--team` (one or more, required), `--lead`, `--start-date`, `--target-date`, `--status` | Mutation `projectCreate` |
| `project update <id>` | `--name`, `--description`, `--status`, `--lead`, `--start-date`, `--target-date`, `--members` | Mutation `projectUpdate` |
| `project delete <id>` | id arg | Mutation `projectDelete` |
| `project unarchive <id>` | id arg | Mutation `projectUnarchive` |

- **`project update` has no `--state` and no `--health` flag.** `ProjectUpdateInput` exposes neither field. A project's state is set through its status: `--status <name>` resolves a `ProjectStatus` name to a `statusId` (via the `projectStatuses` query) and maps to `projectUpdate.statusId` (also `projectCreate.statusId`). Project health is settable only when posting a status update, via `project update-post` (`ProjectUpdateCreateInput.health`); see 4.2.4.
- Archiving uses `projectUpdate` (set `statusId` to a completed/canceled status) or `projectDelete`; do not wire the deprecated `projectArchive`.
- Acceptance: `--team` and `--lead` resolve by key/email; at least one `--team` is required on create and `projectCreate` errors non-zero without it; only changed fields update; `--members` accepts a comma-separated list of emails resolved to IDs; `--status <bad name>` exits non-zero and lists the available project statuses; `--status <name>` sets `statusId` on the mutation. The team, lead, and user (owner/member) resolvers follow Section 3.3: an unknown value exits non-zero with the not-found message, and an ambiguous name exits non-zero and lists each candidate with its UUID.

#### 4.2.2 Project members

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project members <id>` | id arg | Query `project.members` |
| `project member add <id> <user>` / `project member remove <id> <user>` | args | Mutation `projectUpdate` (set `memberIds`) |

- `project member add/remove` follows the standard nested add-child shape (Section 3.1).
- Acceptance: `project members <unknown>` exits non-zero with `Project not found: <name>`; a project with no members prints `No members found` and exits 0. `project member add/remove` read the current member set, apply the delta, and write back `memberIds`, exiting 0 on success. The user resolves by email; an unknown user exits non-zero with the not-found message, and an ambiguous name exits non-zero and lists each candidate with its UUID (Section 3.3).

#### 4.2.3 Project milestones (nested group `project milestone`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project milestone list <project>` | project arg | Query `projectMilestones`, `projectMilestone` |
| `project milestone create <project>` | `--name` (required), `--description`, `--target-date` | Mutation `projectMilestoneCreate` |
| `project milestone update <id>` | `--name`, `--description`, `--target-date` | Mutation `projectMilestoneUpdate` |
| `project milestone move <id>` | `--project`, `--sort-order` | Mutation `projectMilestoneMove` |
| `project milestone delete <id>` | id arg | Mutation `projectMilestoneDelete` |

- Related: `issue create`/`update` gain `--milestone` (see 4.3.5).
- Acceptance: `project milestone list <unknown>` exits non-zero with `Project not found: <name>`; a project with no milestones prints `No milestones found` and exits 0. The milestone resolves by name within a project; an unknown milestone exits non-zero with `Milestone not found: <name>`, and an ambiguous milestone name exits non-zero and lists each candidate with its UUID (Section 3.3). `move` reorders within a project or transfers between projects and exits 0.

#### 4.2.4 Project status updates (nested group `project update-post`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project update-post list <project>` | `--limit` | Query `projectUpdates`, `projectUpdate` (Go op name `GetProjectUpdate`) |
| `project update-post create <project>` | `--body` (required), `--health` | Mutation `projectUpdateCreate` |
| `project update-post edit <post-id>` | `--body`, `--health` | Mutation `projectUpdateUpdate` |
| `project update-post archive <post-id>` / `unarchive <post-id>` | args | Mutation `projectUpdateArchive`, `projectUpdateUnarchive` |
| `project update-reminder <project>` | project arg | Mutation `createProjectUpdateReminder` |

- `--health` maps to `ProjectUpdateCreateInput.health` (`ProjectUpdateHealthType`: `onTrack`, `atRisk`, `offTrack`). This is the only place project health is settable.
- `project update-reminder` creates a reminder to post the next project update; its help states this.
- Archive replaces delete; do not wire the deprecated `projectUpdateDelete`.
- Acceptance: `project update` (edit the project, including `--status`) and `project update-post` (post a status update, including `--health`) stay distinct, and each command's help names both; `--health <bad>` exits non-zero and lists the three values.

#### 4.2.5 Project labels (nested group `project label`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project label list` | `--limit` | Query `projectLabels`, `projectLabel` |
| `project label create` | `--name` (required), `--color`, `--description`, `--parent` | Mutation `projectLabelCreate` |
| `project label update <id>` | `--name`, `--color`, `--description` | Mutation `projectLabelUpdate` |
| `project label delete <id>` | id arg | Mutation `projectLabelDelete` |
| `project label retire <id>` / `restore <id>` | id arg | Mutation `projectLabelRetire`, `projectLabelRestore` |
| `project label add <project> <label>` / `project label remove <project> <label>` | args | Mutation `projectAddLabel`, `projectRemoveLabel` |

- `project label add/remove` uses the standard nested add-child shape (Section 3.1).
- Acceptance: `project label list` with no labels prints `No project labels found` and exits 0. `project label` stays a separate group from the top-level `label` (issue labels). `project label add/remove` resolve project and label by name or ID; an unknown project exits non-zero with `Project not found: <name>` and an unknown label with `Project label not found: <name>`; an ambiguous name exits non-zero and lists each candidate with its UUID (Section 3.3).

#### 4.2.6 Project relations and status

| Command | Key flags / args | Backing operations |
|---|---|---|
| `project relate <id> <other>` | `--relation <kind>`, `--remove` | Mutation `projectRelationCreate`, `projectRelationUpdate`, `projectRelationDelete`; Query `projectRelations` |
| `project status list` | `--limit` | Query `projectStatuses` |
| `project status create` | `--name`, `--color`, `--status-type`, `--position` | Mutation `projectStatusCreate` |
| `project status update <id>` | `--name`, `--color`, `--status-type`, `--position` | Mutation `projectStatusUpdate` |
| `project status archive <id>` / `unarchive <id>` | id arg | Mutation `projectStatusArchive`, `projectStatusUnarchive` |
| `project reassign-status` | `--from`, `--to` | Mutation `projectReassignStatus` |

- `project relate` uses `--relation` (not `--type`) for the relation kind.
- `project status create/update` uses `--status-type` (not `--type`) for `ProjectStatusType` (`backlog`, `planned`, `started`, `paused`, `completed`, `canceled`), per the distinct-kind-flag rule.
- Acceptance: statuses are org-level; `--status-type <bad>` exits non-zero and lists the six type values; `reassign-status` moves all projects from one status to another; relations render in `project get`.

### 4.3 Tier 2 - Cycles, Labels, States, Issue lifecycle, Comments, Documents (PR 3)

New groups: `cmd/cycle.go`, `cmd/label.go`, `cmd/state.go`, `cmd/document.go`; extensions to `cmd/issue.go` and `cmd/comment.go`. New operations: `cycles.graphql`, `labels.graphql`, `states.graphql`, `documents.graphql`, plus additions to `issues.graphql` and `comments.graphql`.

Acceptance criteria follow the concrete, testable shape set in 4.1 (exit codes, exact error text, expected fields).

#### 4.3.1 Cycles (`cmd/cycle.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `cycle list` | `--team`, `--limit`, `--sort`, `--newer-than` (default `all_time`) | Query `cycles`, `cycle` |
| `cycle get <id>` | id arg | Query `cycle` (progress, scope history) |
| `cycle create` | `--team` (required), `--name`, `--starts-at`, `--ends-at` | Mutation `cycleCreate` |
| `cycle update <id>` | `--name`, `--starts-at`, `--ends-at` | Mutation `cycleUpdate` |
| `cycle archive <id>` | id arg | Mutation `cycleArchive` |
| `cycle shift <team>` | `--from`, `--by` | Mutation `cycleShiftAll` |
| `cycle start-now <team>` | team arg | Mutation `cycleStartUpcomingCycleToday` |

- Related: `issue create`/`update` gain `--cycle` (see 4.3.5).
- Acceptance: cycle resolves by number or name within a team; `get` shows progress and scope history; `shift` moves all future cycles.

#### 4.3.2 Issue labels (`cmd/label.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `label list` | `--team`, `--limit` | Query `issueLabels`, `issueLabel` |
| `label create` | `--name` (required), `--color`, `--description`, `--parent`, `--team` | Mutation `issueLabelCreate` |
| `label update <id>` | `--name`, `--color`, `--description` | Mutation `issueLabelUpdate` |
| `label delete <id>` | id arg | Mutation `issueLabelDelete` |
| `label retire <id>` / `restore <id>` | id arg | Mutation `issueLabelRetire`, `issueLabelRestore` |

- Related: `issue create`/`update` gain `--label` (see 4.3.5), backed by `issueAddLabel`/`issueRemoveLabel` or `labelIds`.
- Acceptance: parent group resolves by name; workspace and team-scoped labels both list.

#### 4.3.3 Workflow states (`cmd/state.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `state list <team>` | team arg | Query `GetTeamStates` (existing embed) |
| `state get <id>` | id arg | Query `workflowState` |
| `state create <team>` | `--name` (required), `--state-type`, `--color`, `--position` | Mutation `workflowStateCreate` |
| `state update <id>` | `--name`, `--state-type`, `--color`, `--position` | Mutation `workflowStateUpdate` |
| `state delete <id>` | id arg | Mutation `workflowStateArchive` |

- `state list` reuses the existing `GetTeamStates` query (the same embed the state resolver uses) rather than a new operation.
- `state get` and `state delete` complete CRUD parity. Linear has no hard-delete for workflow states, so `state delete` maps to `workflowStateArchive`; the command help states that delete archives the state.
- `state create/update` uses `--state-type` (not `--type`) for the workflow state type (`backlog`, `unstarted`, `started`, `completed`, `canceled`).
- Acceptance: `--state-type <bad>` exits non-zero and lists the valid values; state resolves by name within a team.

#### 4.3.4 Issue lifecycle (extend `cmd/issue.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `issue archive <id>` / `issue unarchive <id>` | id arg | Mutation `issueArchive`, `issueUnarchive` |
| `issue delete <id>` | id arg | Mutation `issueDelete` |
| `issue subscribe <id>` / `issue unsubscribe <id>` | id arg | Mutation `issueSubscribe`, `issueUnsubscribe` |
| `issue remind <id>` | `--at` | Mutation `issueReminder` |
| `issue share <id>` / `issue unshare <id>` | id arg | Mutation `issueShare`, `issueUnshare` |

- Acceptance: `archive` removes the README "coming soon" note; `share` prints the public URL; `unshare` revokes it.

#### 4.3.5 Richer issue create/update and bulk (extend `cmd/issue.go`)

| Change | Flags added | Backing operations |
|---|---|---|
| Enrich `issue create`/`issue update` | `--label` (repeatable), `--cycle`, `--estimate`, `--parent`, `--milestone`; on create also `--project`, `--assignee`, `--state`, `--due-date` | `issueAddLabel`/`issueRemoveLabel` or `labelIds` in `IssueCreateInput`/`IssueUpdateInput` |
| `issue batch-create` | `--file` (JSON/CSV) or repeatable `--title` | Mutation `issueBatchCreate` |
| `issue batch-update` | `--filter` or id list plus set flags | Mutation `issueBatchUpdate` |

- The create/update helper queries `issuePriorityValues`, `issueFilterSuggestion`, `issueVcsBranchSearch`, and `issueRepositorySuggestions` are internals that enrich these commands, not standalone commands (see 4.6.7).
- **Resolver caching:** batch commands resolve each referenced team, label, cycle, milestone, and assignee once per invocation and reuse the cached result across rows (Section 3.3), so a large batch does not re-resolve per row.
- **Backoff:** batch commands retry on rate-limit errors with client-side backoff (Section 3.7).
- **Accepted risk:** `issue batch-update` executes immediately with no client-side max-batch guard (Section 3.6).
- **Regression:** the new flags carry a flag-regression check (Section 3.8) proving existing issue flags still work.
- Acceptance: labels, cycle, milestone, parent, and assignee resolve by name/email; batch operations report per-item success and failure counts, following the `attachment upload` summary pattern.

#### 4.3.6 Comments (extend `cmd/comment.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `comment edit <id>` | `--body` | Mutation `commentUpdate` (already defined in `operations/comments.graphql`, wire it) |
| `comment delete <id>` | id arg | Mutation `commentDelete` |
| `comment resolve <id>` / `comment unresolve <id>` | id arg | Mutation `commentResolve`, `commentUnresolve` |
| `comment create` gains `--parent` | `--parent <comment-id>` | existing `commentCreate` with `parentId` |
| `comment react <id>` / `issue react <id>` | `--emoji`, `--remove` | Mutation `reactionCreate`, `reactionDelete` |

- Acceptance: reply threads via `--parent`; `react` toggles with `--remove`; reactions also apply to issues (shared with 4.5.3).

#### 4.3.7 Documents (`cmd/document.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `document list` | `--limit`, `--project`, `--initiative`, `--newer-than` (default `all_time`) | Query `documents`, `document` |
| `document search <term>` | term arg | Query `searchDocuments` |
| `document get <id>` | id arg | Query `document`, `documentContentHistory` |
| `document create` | `--title` (required), `--content`, `--project` or `--initiative` | Mutation `documentCreate` |
| `document update <id>` | `--title`, `--content` | Mutation `documentUpdate` |
| `document delete <id>` / `document unarchive <id>` | id arg | Mutation `documentDelete`, `documentUnarchive` |

- Acceptance: `create` requires exactly one of `--project`/`--initiative`; `get` can show content history.

### 4.4 Tier 3 - Teams, Users/Org, Favorites, Views, Notifications, Webhooks, Templates (PR 4)

Extensions to `cmd/team.go`, `cmd/user.go`; new groups `cmd/org.go`, `cmd/favorite.go`, `cmd/view.go`, `cmd/notification.go`, `cmd/webhook.go`, `cmd/template.go`.

#### 4.4.1 Teams (extend `cmd/team.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `team create` | `--name` (required), `--key`, `--description` | Mutation `teamCreate` |
| `team update <key>` | `--name`, `--description`, and settings flags | Mutation `teamUpdate` |
| `team delete <key>` / `team unarchive <key>` | key arg | Mutation `teamDelete`, `teamUnarchive` |
| `team member add <key> <user>` / `team member remove <key> <user>` / `team set-role <key> <user>` | `--role` | Mutation `teamMembershipCreate`, `teamMembershipDelete`, `teamMembershipUpdate` |

- `team member add/remove` uses the standard nested add-child shape (Section 3.1).
- `team set-role` sets a **team-membership** role; its help says so explicitly to distinguish it from `user set-role` (org role).
- **`team delete` executes immediately with no prompt** (accepted risk, Section 3.6).
- Acceptance: membership commands resolve team by key and user by email; `set-role` updates the membership role.

#### 4.4.2 Users and organization (extend `cmd/user.go`, new `cmd/org.go`)

| Command | Key flags / args | Backing operations |
|---|---|---|
| `user update` | profile flags (`--name`, `--display-name`, and so on) | Mutation `userUpdate` |
| `user set-role <id>` | `--role owner\|admin\|guest\|user\|app` | Mutation `userChangeRole` |
| `user suspend <id>` / `user unsuspend <id>` | id arg | Mutation `userSuspend`, `userUnsuspend` |
| `org get` | none | Query `organization` |
| `org update` | settings flags | Mutation `organizationUpdate` |
| `org invite list` | `--limit` | Query `organizationInvites` |
| `org invite create` | `--email`, `--role` | Mutation `organizationInviteCreate` |
| `org invite delete <id>` / `org invite resend <id>` | id arg | Mutation `organizationInviteDelete`, `resendOrganizationInvite` |

- `userChangeRole` replaces the removed `userPromoteAdmin`/`userDemoteAdmin`/`userPromoteMember`/`userDemoteMember`.
- `user set-role` sets an **org** role (`UserRoleType`: `owner`, `admin`, `guest`, `user`, `app`); its help says so to distinguish it from `team set-role`. `member` is not a valid org role; it belongs to `TeamRoleType` (used by `team set-role`).
- **`user suspend` executes immediately with no prompt** (accepted risk, Section 3.6).
- Acceptance: `user update` edits only the viewer's profile; `--role <bad>` exits non-zero and lists `owner`, `admin`, `guest`, `user`, `app`.

#### 4.4.3 Favorites, views, notifications, webhooks, templates

| Domain | Commands | Backing operations |
|---|---|---|
| Favorites (`cmd/favorite.go`) | `favorite list`; `favorite add`/`favorite remove` with `--entity <kind>` for issues, projects, cycles, and more | Query `favorites`, `favorite`; Mutation `favoriteCreate`, `favoriteUpdate`, `favoriteDelete` |
| Custom views (`cmd/view.go`) | `view list`; `view create`/`update`/`delete` | Query `customViews`, `customView`; Mutation `customViewCreate`, `customViewUpdate`, `customViewDelete` |
| Notifications (`cmd/notification.go`) | `notification list`; `read-all`/`unread-all`; `archive`/`snooze` | Query `notifications`, `notification`, `notificationsUnreadCount`; Mutation `notificationMarkReadAll`, `notificationMarkUnreadAll`, `notificationArchive`, `notificationArchiveAll`, `notificationSnoozeAll` |
| Notification subscriptions | `notification subscribe`/`unsubscribe` (deactivate via update) | Mutation `notificationSubscriptionCreate`, `notificationSubscriptionUpdate` (avoid deprecated `notificationSubscriptionDelete`; set `active=false`) |
| Webhooks (`cmd/webhook.go`) | `webhook list`/`get`; `create`/`update`/`delete`; `rotate-secret` | Query `webhooks`, `webhook`; Mutation `webhookCreate`, `webhookUpdate`, `webhookDelete`, `webhookRotateSecret` |
| Templates (`cmd/template.go`) | `template list` (issue and project); `create`/`update`/`delete`; apply on create | Query `templates`, `template`; Mutation `templateCreate`, `templateUpdate`, `templateDelete` |

- `favorite add/remove` uses `--entity <kind>` (not `--type`) to pick the favorited entity type, per the distinct-kind-flag rule.
- `webhook create` validates the target URL before calling the API (Section 5.4): reject `localhost`, loopback, link-local, and private/internal IP ranges, because Linear fetches the URL server-side.
- **`webhook rotate-secret` prints the new secret once** (accepted risk, Section 3.6).
- Acceptance: `notification list` shows the unread count; template `apply` threads a `templateId` into `issue create`/`project create` (a cross-tier touch on those files, see Section 7).

### 4.5 Tier 4 - Advanced domains (PRs 5a, 5b, 5c)

New groups: `cmd/release.go`, `cmd/customer.go`, `cmd/emoji.go`, `cmd/triage.go`, `cmd/git.go`, `cmd/schedule.go`, `cmd/oauthapp.go`, `cmd/agent.go`; plus `attachment link` and `attachment sync-to-slack` on the existing group. New operations per domain file. Tier 4 splits across three PRs so no single PR dwarfs the others (Section 6):

- **PR 5a:** Releases (4.5.1), Customers/CRM (4.5.2).
- **PR 5b:** Reactions/Emoji (4.5.3), Triage, git automation, and schedules (4.5.4).
- **PR 5c:** OAuth apps (4.5.5), Attachment link helpers (4.5.6), Agents API last (4.5.7).

**Plan-availability gate (mandatory, per domain).** Before building any Tier 4 domain, verify it is available for writes on the current plan by attempting one scoped write in the sandbox (Section 3.8). Read access works across all domains (initiatives, customers, releases, agents, and so on), but write availability for enterprise features (Releases, Customers/CRM) is unproven on this plan. The team-creation cap (Free/Standard tier, 5-team limit reached, so no dedicated test team) is direct evidence that plan limits bite here. If a scoped write returns a plan or permission error, defer that domain and record the finding in the PR rather than shipping commands that always fail.

#### 4.5.1 Releases (`cmd/release.go`) - PR 5a

| Sub-group | Commands | Backing operations |
|---|---|---|
| release | `release list`/`get`/`search` (`--newer-than` default `all_time`); `create`/`update`/`delete`; `archive`/`unarchive`/`complete`; `sync` | Query `releases`, `release`, `releaseSearch`; Mutation `releaseCreate`, `releaseUpdate`, `releaseDelete`, `releaseArchive`, `releaseUnarchive`, `releaseComplete`, `releaseSync`, `releaseUpdateByPipeline` |
| release pipeline | `list`/`create`/`update`/`delete`/`archive`/`unarchive` | Query `releasePipelines`, `releasePipeline`; Mutation `releasePipelineCreate`, `releasePipelineUpdate`, `releasePipelineDelete`, `releasePipelineArchive`, `releasePipelineUnarchive` |
| release stage | `list`/`create`/`update`/`archive`/`unarchive` | Query `releaseStages`, `releaseStage`; Mutation `releaseStageCreate`, `releaseStageUpdate`, `releaseStageArchive`, `releaseStageUnarchive` |
| release note | `list`/`create`/`update`/`delete` | Query `releaseNotes`, `releaseNote`; Mutation `releaseNoteCreate`, `releaseNoteUpdate`, `releaseNoteDelete` |
| issue-to-release links | `release issue add`/`release issue remove` | Query `issueToReleases`, `issueToRelease`; Mutation `issueToReleaseCreate`, `issueToReleaseDelete`, `issueToReleaseDeleteByIssueAndRelease` |

- Access-key variants (`releaseCompleteByAccessKey`, `releaseSyncByAccessKey`, `latestReleaseByAccessKey`, `recentReleasesByAccessKey`, `releasePipelineByAccessKey`) stay unwired unless a concrete CI use case appears; note them as a deferred sub-option.
- Output: `release get` nests pipeline then stages (Section 3.4), with summary counts plus an indented sub-table in table mode. Acceptance: pipeline and stage resolve by name within a release; `complete` marks a release done.

#### 4.5.2 Customers / CRM (`cmd/customer.go`) - PR 5a

| Sub-group | Commands | Backing operations |
|---|---|---|
| customer | `list`/`get` (`--newer-than` default `all_time`); `create`/`update`/`delete`/`merge`/`upsert`/`unsync` | Query `customers`, `customer`; Mutation `customerCreate`, `customerUpdate`, `customerDelete`, `customerMerge`, `customerUpsert`, `customerUnsync` |
| customer need | `list`/`get`/`create`/`update`/`delete`/`archive`/`unarchive`; `create-from-attachment` | Query `customerNeeds`, `customerNeed`; Mutation `customerNeedCreate`, `customerNeedUpdate`, `customerNeedDelete`, `customerNeedArchive`, `customerNeedUnarchive`, `customerNeedCreateFromAttachment` |
| customer status | `list`/`create`/`update`/`delete` | Query `customerStatuses`, `customerStatus`; Mutation `customerStatusCreate`, `customerStatusUpdate`, `customerStatusDelete` |
| customer tier | `list`/`create`/`update`/`delete` | Query `customerTiers`, `customerTier`; Mutation `customerTierCreate`, `customerTierUpdate`, `customerTierDelete` |

- **`customer merge` executes immediately with no prompt** (accepted risk, Section 3.6): it takes two customers and reports the survivor.
- `customer` output can carry PII; `--json` prints it raw (Section 5.4).
- Acceptance: `merge` reports the survivor; `upsert` keys on an external identifier.

#### 4.5.3 Reactions and emoji (`cmd/emoji.go`, shared reactions) - PR 5b

| Command | Key flags / args | Backing operations |
|---|---|---|
| `issue react`/`comment react` (from 4.3.6) | `--emoji`, `--remove` | Mutation `reactionCreate`, `reactionDelete` |
| `emoji list` | `--limit` | Query `emojis`, `emoji` |
| `emoji create` | `--name`, `--url` | Mutation `emojiCreate` |
| `emoji delete <id>` | id arg | Mutation `emojiDelete` |

- Acceptance: custom org emoji list/create/delete work; reactions share one helper across issues and comments.

#### 4.5.4 Triage, git automation, schedules - PR 5b

| Domain | Commands | Backing operations |
|---|---|---|
| Triage (`cmd/triage.go`) | `triage list`/`get`/`create`/`update`/`delete` | Query `triageResponsibilities`, `triageResponsibility`; Mutation `triageResponsibilityCreate`, `triageResponsibilityUpdate`, `triageResponsibilityDelete` |
| Git automation (`cmd/git.go`) | `git state create`/`update`/`delete`; `git target-branch create`/`update`/`delete` | Mutation `gitAutomationStateCreate`, `gitAutomationStateUpdate`, `gitAutomationStateDelete`, `gitAutomationTargetBranchCreate`, `gitAutomationTargetBranchUpdate`, `gitAutomationTargetBranchDelete` |
| Time schedules (`cmd/schedule.go`) | `schedule list`/`get`; `create`/`update`/`delete`; `upsert-external`/`refresh` | Query `timeSchedules`, `timeSchedule`; Mutation `timeScheduleCreate`, `timeScheduleUpdate`, `timeScheduleDelete`, `timeScheduleUpsertExternal`, `timeScheduleRefreshIntegrationSchedule` |

- Acceptance: the `schedule` group name is honored; schedules resolve by name or ID.

#### 4.5.5 OAuth apps (`cmd/oauthapp.go`) - PR 5c

| Domain | Commands | Backing operations |
|---|---|---|
| OAuth apps | `oauth-app list`/`get`; `create`/`update`/`archive`; `rotate-secret`/`rotate-webhook-secret` | Query `oauthApplications`, `oauthApplication`; Mutation `oauthApplicationCreate`, `oauthApplicationUpdate`, `oauthApplicationArchive`, `oauthApplicationRotateSecret`, `oauthApplicationRotateWebhookSecret` |

- **`rotate-secret` and `rotate-webhook-secret` execute immediately and print the new secret once** (accepted risk, Section 3.6).
- Acceptance: the `oauth-app` group name is honored; rotate commands print the new secret once.

#### 4.5.6 Attachment link helpers (extend `cmd/attachment.go`) - PR 5c

| Command | Key flags / args | Backing operations |
|---|---|---|
| `attachment link <issue> <url>` | `--provider <name>` | one command dispatching to the typed mutations `attachmentLinkURL`, `attachmentLinkSlack`, `attachmentLinkDiscord`, `attachmentLinkFront`, `attachmentLinkIntercom`, `attachmentLinkZendesk`, `attachmentLinkJiraIssue`, `attachmentLinkGitHubIssue`, `attachmentLinkGitHubPR`, `attachmentLinkGitLabMR`, `attachmentLinkSalesforce` |
| `attachment sync-to-slack <id>` | id arg | Mutation `attachmentSyncToSlack` |

- `--provider` (not `--type`) selects the provider; default `url`. A single command wraps the roughly 11 typed link mutations behind one interface, per the confirmed refinement.
- Acceptance: each provider routes to its mutation; an unknown provider exits non-zero and lists the valid providers.

#### 4.5.7 Agents API (`cmd/agent.go`, lowest build priority) - PR 5c

| Sub-group | Commands | Backing operations |
|---|---|---|
| agent session | `list`/`get`; `create`/`create-on-issue`/`create-on-comment`; `update`/`update-external-url`; `sandbox` | Query `agentSessions`, `agentSession`, `agentSessionSandbox`; Mutation `agentSessionCreate`, `agentSessionCreateOnIssue`, `agentSessionCreateOnComment`, `agentSessionUpdate`, `agentSessionUpdateExternalUrl` |
| agent activity | `list`/`get`; `create`/`create-prompt`/`send-queued`/`delete-queued` | Query `agentActivities`, `agentActivity`; Mutation `agentActivityCreate`, `agentActivityCreatePrompt`, `agentActivitySendQueued`, `agentActivityDeleteQueued` |
| agent skill | `list`/`get`; `create`/`update`/`delete` | Query `agentSkills`, `agentSkill`; Mutation `agentSkillCreate`, `agentSkillUpdate`, `agentSkillDelete` |

- Build the Agents API last within PR 5c and confirm the use case first. It targets AI-agent integrations, not general CRUD. A review decision kept it in scope at the lowest priority (Section 8, Resolved during review).

### 4.6 Tier 5 - Additional operations (PR 6)

Small, cross-cutting additions surfaced by the full audit. Some extend existing groups; some add tiny new ones.

| # | Area | Command(s) | Backing operations |
|---|---|---|---|
| 4.6.1 | Audit log (`cmd/audit.go`) | `audit list` | Query `auditEntries`, `auditEntryTypes` |
| 4.6.2 | Global search (extend `cmd/search` surface) | `search projects <term>`; `search semantic <term>` | Query `searchProjects`, `semanticSearch` |
| 4.6.3 | Diagnostics (`cmd/ratelimit.go`) | `rate-limit` | Query `rateLimitStatus` |
| 4.6.4 | External links (`cmd/link.go`) | `link add`/`update`/`remove` on any entity, including initiatives and documents | Query `entityExternalLink`; Mutation `entityExternalLinkCreate`, `entityExternalLinkUpdate`, `entityExternalLinkDelete` |
| 4.6.5 | View preferences (extend `cmd/view.go`) | `view prefs create`/`update`/`delete` | Mutation `viewPreferencesCreate`, `viewPreferencesUpdate`, `viewPreferencesDelete` |
| 4.6.6 | User settings (extend `cmd/user.go`) | `user settings update` | Mutation `userSettingsUpdate` |
| 4.6.7 | Create/update helpers (internals, not standalone commands) | enrich `issue create`/`update` and branch tooling | Query `issuePriorityValues`, `issueFilterSuggestion`, `issueVcsBranchSearch`, `issueRepositorySuggestions` |
| 4.6.8 | Issue relation update | `issue relate` gains update support | Mutation `issueRelationUpdate` |
| 4.6.9 | SLA and external users (read) | `sla list`; `user external list`/`get` | Query `slaConfigurations`, `externalUsers`, `externalUser` |

- **`rate-limit` must query the real `rateLimitStatus`.** The existing `Client.GetRateLimit()` placeholder returns fake data; remove or replace it and wire `rate-limit` to `rateLimitStatus`, so the command reports the actual remaining budget rather than a hardcoded value.
- The Tier 5 helper queries in 4.6.7 are wired as internals: they inform `issue create`/`update` (priority values, filter suggestions) and branch tooling (VCS branch search, repository suggestions). They do not get their own top-level commands.
- `issue relate` uses `--relation` (not `--type`), consistent with 4.1.3 and 4.2.6.
- **SLA is a Business+ feature.** Run the plan-availability gate (Section 4.5) before building `sla list`: attempt one `slaConfigurations` read in the sandbox, and defer the command if the plan returns a plan or permission error, recording the finding in the PR.
- Audit entries, external users, and invites carry PII; `--json` prints them raw (Section 5.4).
- Acceptance: `rate-limit` shows the current budget from `rateLimitStatus`; `link` works generically across entity types; `search semantic` returns AI results; SLA and external-user commands are read-only.

---

## 5. Cross-Cutting Concerns

### 5.1 Backward compatibility

- Existing commands (`auth`, `issue`, `project`, `team`, `user`, `comment`, `attachment`) keep their current flags, output, and behavior. New flags on `issue create`/`update` and `comment create` are additive and default to no-op when unset.
- The 48 existing smoke tests must keep passing unchanged at every PR. New tests append; they do not modify existing assertions.
- The new `issue create`/`update` flags carry a flag-regression check (Section 3.8) proving existing issue flags still work when the new flags are unset and when combined.
- Enriching `issue update` (the in-flight `--project` WIP in `cmd/issue.go` and `pkg/api/client.go`) lands as its own baseline commit before Tier work, per the discovery pre-work note.

### 5.2 Schema and codegen impact

- Each tier adds `.graphql` operation files and regenerates `pkg/api/generated.go` via `go generate ./pkg/api`. Never edit `generated.go` by hand.
- The schema is already refreshed to the live API (`scripts/update-schema.sh`), so no schema pull is needed mid-project; regenerate bindings only.
- After each regeneration, run `make build` and fix any type mismatches before wiring commands.
- Watch for genqlient operation-name collisions and disambiguate at definition time (Section 3.2), notably `initiativeUpdate` and `projectUpdate` (mutation edits the parent, query fetches one status update).
- Remove or replace the dead `Client.GetRateLimit()` placeholder when wiring the Tier 5 `rate-limit` command (Section 4.6.3).

### 5.3 Documentation updates

- `README.md`: add a command reference section per new domain, remove the "coming soon" note for `issue archive`, and document the three output modes on new commands.
- `CLAUDE.md`: extend the "Adding Commands" and "AI Agent Notes" sections with the new groups and the name-to-ID resolver conventions, including the recommendation to pass IDs in scripts and agent calls.
- `cmd/CLAUDE.local.md` and `pkg/api/CLAUDE.local.md`: note the shared `cmd/resolve.go` resolver helpers, the per-invocation caching rule, and the operation-naming rule for collision cases.
- Each PR updates docs for the domains it ships, so documentation stays in lockstep with code.

### 5.4 Security and data handling

Operations stay unconfirmed and un-redacted, so these behaviors are documented rather than fixed.

- **`--json` prints raw structs, including PII and secrets.** Customer records, audit entries, users, and invites can contain personal data, and secret-rotation commands print the new secret in their payload. This is accepted for a personal CLI. A future optional `--redact` flag (Deferred suggestions) could mask sensitive fields for shared output.
- **Secret-rotation output** appears once on stdout; the caller handles shell history and logs (Section 3.6).
- **Webhook URL validation.** `webhook create` validates the target URL before calling the API and rejects `localhost`, loopback, link-local, and private/internal IP ranges, because Linear fetches the URL server-side and an internal URL invites server-side request forgery.

---

## 6. Rollout / Phasing Plan

Delivery follows the discovery decision: build in tier order, each PR a working and tested checkpoint. Tier 4 splits into three PRs so no PR dwarfs the others. The design is specified up front, but code lands incrementally. Effort is a rough t-shirt size derived from operation count and domain complexity.

| PR | Tier | Domains | Primary new files | Effort |
|---|---|---|---|---|
| Pre-work | baseline | Commit gap doc and brainstorm docs; commit `issue update --project` WIP separately; create feature branch; verify baseline suite | none | S |
| PR 1 | Tier 1a | Initiatives (CRUD, project links, labels, relations, status updates) | `cmd/initiative.go`, `operations/initiatives.graphql` | M |
| PR 2 | Tier 1b | Projects (CRUD, members, milestones, status updates, labels, relations, status) | extend `cmd/project.go`, `operations/projects.graphql` | M |
| PR 3 | Tier 2 | Cycles, issue labels, workflow states, issue lifecycle and richer create/update and bulk, comments, documents | `cmd/cycle.go`, `cmd/label.go`, `cmd/state.go`, `cmd/document.go`, extend `cmd/issue.go`, `cmd/comment.go` | L |
| PR 4 | Tier 3 | Teams, users/org, favorites, views, notifications, webhooks, templates | extend `cmd/team.go`, `cmd/user.go`; add `cmd/org.go`, `cmd/favorite.go`, `cmd/view.go`, `cmd/notification.go`, `cmd/webhook.go`, `cmd/template.go` | L |
| PR 5a | Tier 4a | Releases, customers/CRM | `cmd/release.go`, `cmd/customer.go` | L |
| PR 5b | Tier 4b | Reactions/emoji, triage, git automation, schedules | `cmd/emoji.go`, `cmd/triage.go`, `cmd/git.go`, `cmd/schedule.go` | M |
| PR 5c | Tier 4c | OAuth apps, attachment link helpers, agents (last) | `cmd/oauthapp.go`, extend `cmd/attachment.go`, `cmd/agent.go` | M |
| PR 6 | Tier 5 | Audit, global search, rate-limit, external links, view preferences, user settings, helper-query internals, issue relation update, SLA and external users | `cmd/audit.go`, `cmd/link.go`, `cmd/ratelimit.go`, extend `cmd/user.go`, `cmd/view.go`, `cmd/issue.go` | M |

**PR 5a is the heaviest PR** despite its "L" size: ~80 operations across Releases and Customers/CRM, more than any other PR. Track its schedule closely and split it further (Releases and Customers as separate PRs) if it slips.

Per-PR checklist:

1. Define operations, run `go generate ./pkg/api`, `make build`.
2. For Tier 4 PRs, run the plan-availability gate first (Section 4.5): attempt one scoped write per domain in the sandbox and defer any domain that returns a plan or permission error.
3. Wire commands with full three-mode output parity.
4. Add read-only smoke tests; keep the suite green.
5. Manually verify write commands against the `lincli-sandbox` project (and the SD team for team-scoped writes), recording each command's run plus expected and actual output in the PR write-verification log (Section 3.8).
6. Update `README.md` and the CLAUDE docs for the shipped domains.

Shared groundwork (the `cmd/resolve.go` resolver helpers, including per-invocation caching) lands in PR 1 and grows as later tiers add entity types.

---

## 7. Risks and Mitigations

| Risk | Category | Impact | Likelihood | Mitigation |
|---|---|---|---|---|
| genqlient operation-name collisions (`initiativeUpdate`, `projectUpdate` mutation vs status-update query) | Technical | Build breaks or wrong operation wired | High | Give each operation a unique, descriptive Go name at definition (Section 3.2); review generated function names after codegen |
| Destructive commands run with no prompt, including high-blast-radius ops | Operational | Lost or altered data during manual write testing | Medium | Accepted risk (Section 3.6); confine write verification to `lincli-sandbox` and the SD team (Section 3.8); never run destructive automated tests on live data |
| Enterprise write features unavailable on the current plan | Dependency | Tier 4 commands ship but always fail | Medium | Plan-availability gate before each Tier 4 domain (Section 4.5); the team-cap finding shows plan limits are real; defer unavailable domains |
| Deprecated write paths chosen by mistake | Technical | Silent deprecation warnings, future breakage | Medium | Route around deprecated ops explicitly (Section 3.6); document the preferred path per domain |
| Deep nested `DetailFields` fragments exceed query-depth/complexity limits | Technical | `get` requests rejected | Medium | Fall back to separate paginated calls per nested section when a deep query is rejected (Section 3.4) |
| Nested rendering inconsistent across modes | Technical | Confusing output, agent parse failures | Medium | Define nested-section rules once (Section 3.4); reuse fragments; assert JSON shape in review |
| Name-to-ID ambiguity (duplicate names) | Technical | Wrong entity mutated | Medium | Prefer exact match, report ambiguity with candidate IDs, accept raw UUIDs everywhere (Section 3.3) |
| Cross-tier rework on shared files | Schedule | Repeated churn, merge conflicts | Medium | `issue create`/`update` gains flags across PR 1-3 and templates thread a `templateId` into `issue create`/`project create` in PR 4; these files reopen in later PRs. Keep flag wiring additive, land the `issue update` baseline first (Section 5.1), and re-run the flag-regression check each time |
| Scope size (~330 operations) stalls a PR | Schedule | Slow, hard-to-review PRs | High | Tier order with Tier 4 split into 5a/5b/5c; each PR is an independent working checkpoint; sub-groups reviewable in isolation (Section 6) |
| Rate limiting during batch operations and testing | Dependency | Failed runs | Medium | Per-invocation resolver caching and client-side backoff for bulk ops (Sections 3.3, 3.7); ship `rate-limit` (Tier 5) to inspect the budget; keep the 5,000 req/hour cap in mind |
| Schema drift between now and later tiers | Dependency | Codegen mismatch | Low | Schema already refreshed; regenerate bindings per tier; re-pull only if the API changes |
| Agents API use case unclear | Product | Wasted effort | Medium | Build it last in PR 5c at the lowest priority and confirm the use case first (kept in scope per Section 8, Resolved during review) |
| PII or secrets exposed via `--json` | Security | Leaked personal data or credentials | Low | Accepted for a personal CLI; document the exposure and suggest a future `--redact` (Section 5.4); rotated secrets print once to stdout only |
| Webhook URL points at an internal address | Security | Server-side request forgery via Linear's fetch | Low | Validate `webhook create` URLs and reject localhost/loopback/link-local/internal ranges (Section 5.4) |

---

## 8. Open Questions

| # | Question | Owner | Notes |
|---|---|---|---|
| 1 | Do any CI workflows need the release access-key variants (`releaseCompleteByAccessKey`, etc.)? | Shane | Wire only if a concrete unauthenticated pipeline use case appears |

Resolved during this revision: the write-verification target (now the `lincli-sandbox` project plus the SD team, Section 3.8); name-to-ID resolution (now the concrete rule in Section 3.3); and the favorite/view type flag (now `favorite --entity`, Section 4.4.3).

Resolved during review: the Agents API (4.5.7) stays in scope, built last as PR 5c at the lowest priority; it is no longer an open question.

---

## Appendix A: Coverage Map (tier -> domain -> PR)

| Tier | Domains | Approx. operations | PR |
|---|---|---|---|
| 1 | Initiatives | ~20 | PR 1 |
| 1 | Projects | ~30 | PR 2 |
| 2 | Cycles, Labels, States, Issue lifecycle/create/bulk, Comments, Documents | ~50 | PR 3 |
| 3 | Teams, Users/Org, Favorites, Views, Notifications, Webhooks, Templates | ~55 | PR 4 |
| 4 | Releases, Customers/CRM | ~80 | PR 5a |
| 4 | Reactions/Emoji, Triage, Git automation, Schedules | ~25 | PR 5b |
| 4 | OAuth apps, Attachment links, Agents | ~25 | PR 5c |
| 5 | Audit, Global search, Rate-limit, External links, View preferences, User settings, Helper internals, Issue relation update, SLA, External users | ~25 | PR 6 |

PR 5a carries the largest operation count (~80); track it closely and split Releases and Customers into separate PRs if it slips (see Section 6).

All operation names cited in Section 4 map directly to `.graphql` files during the plan step. The counts are approximate and include read, detail, and suggestion siblings pulled in with their parents.

## Appendix B: Convention Quick Reference

- Output modes: `output.JSON`, `output.Error`, `output.Success`, `output.Info`, `output.Table` (three modes each).
- Nullable clearing: `api.NullSentinel` -> JSON `null` via `stripNulls`.
- Nullable reads: `derefStr(*string)`.
- Optional input fields: set only when `cmd.Flags().Changed("...")`.
- Filters: `build<Domain>FilterTyped(cmd)` returning a typed `api.<Domain>Filter`.
- Resolvers: shared in `cmd/resolve.go`; every resolver accepts a raw UUID unchanged, caches its lookups within one command invocation, and is bounded to the single workspace of the configured API key.
- Add-child shape: `<parent> <child> add/remove`; kind flags are domain-specific (`--relation`, `--provider`, `--entity`, `--state-type`, `--status-type`), never a shared `--type`.
- Codegen: define in `operations/*.graphql`, run `go generate ./pkg/api`, then `make build`.
- Tests: read-only smoke tests in `smoke_test.sh`; write ops verified manually against `lincli-sandbox` (project scope) or the SD team (team scope), logged in the PR.

## Appendix C: Deferred suggestions

These SUGGESTION-level items are out of scope for this revision but recorded so they are not lost:

- Add a CI check for genqlient operation-name and fragment-name collisions.
- Add a mocked GraphQL layer to support resolver unit tests.
- Add an optional `--redact` flag to mask PII and secrets in output.
- Mandate an Examples block in every command's help.
- Unify the search command shape across `document search`, `release search`, and the Tier 5 `search` surface.
- Assign a per-tier sign-off owner in the rollout plan.
- Add person-day estimates alongside the t-shirt sizes in Section 6.

---

*Generated from the brainstorm workflow (step 03). Source of truth for scope: `FEATURE-GAP-ANALYSIS.md`.*

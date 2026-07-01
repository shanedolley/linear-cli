# lincli Feature Gap Analysis

This document compares the current `lincli` command surface against the **live** Linear
GraphQL API (introspected directly, not from the checked-in schema). It lists every
capability the API exposes that the CLI does not yet cover.

**How to use it:** tick the checkbox next to any capability you want scoped for
development. Sections are ordered by priority. Initiatives and Projects come first, since
those are the areas that have forced fallback to the raw GraphQL API.

## Legend

- `[ ]` capability to consider - tick to request a build
- **Mutation/Query** names are the exact Linear GraphQL operations that back each command
- Proposed command names are suggestions, not final

## Snapshot: what exists today

| Domain | Commands present | Write coverage |
|---|---|---|
| auth | `login`, `status`, `logout`, `whoami` | full |
| issue | `list`, `search`, `get`, `create`, `update`, `assign`, `link` | partial |
| project | `list`, `get` | read only |
| team | `list`, `get`, `members` | read only |
| user | `list`, `get`, `me` | read only |
| comment | `list`, `create` | partial |
| attachment | `list`, `create`, `upload`, `update`, `delete` | full |
| initiative | none | none |
| cycle | none (shown inside `issue get`) | none |
| label | none (shown inside `issue get`) | none |
| document | none (shown inside `project get`) | none |
| workflow state | read only (inside `team get`) | none |
| everything else | none | none |

The live API exposes **353 mutations** and **153 queries**. The CLI wires up roughly 15 of
them.

## Schema freshness (now refreshed)

The checked-in `pkg/api/schema.graphql` was a November 2025 snapshot. It has since been
refreshed from the live API with `scripts/update-schema.sh`, the genqlient bindings were
regenerated, and the build plus all 48 smoke tests pass. The analysis below reflects the
current live surface.

The refresh added roughly **62 operations** and reshaped a few. Highlights:

- **Releases** is a new domain (~30 operations) that did not exist in November.
- **Initiative labels** (`initiativeAddLabel`, `initiativeRemoveLabel`) and
  `initiativeLeadTeamUpdate` are new.
- **Roadmaps are deprecated** in favor of Initiatives. `roadmapCreate`, `roadmapUpdate`,
  `roadmapDelete`, and the roadmap queries still resolve but carry deprecation notices. Do
  not build roadmap commands.
- `userPromoteAdmin`, `userDemoteAdmin`, `userPromoteMember`, and `userDemoteMember` were
  removed and replaced by a single `userChangeRole` mutation.
- Several write paths are now deprecated in favor of alternatives: `projectArchive` (use
  `projectDelete`, or set state through `projectUpdate`), `projectUpdateDelete` (use
  `projectUpdateArchive`), `notificationSubscriptionDelete` (set `active=false`), and
  `attachmentIssue` as a query (use `attachmentsForURL`).

---

## Tier 1 - Initiatives

Initiatives have **zero** command coverage today. This is the largest single gap.

### Initiative CRUD
- [x] `initiative list` - list initiatives (**Query** `initiatives`)
- [x] `initiative get <id>` - initiative detail with linked projects (**Query** `initiative`)
- [x] `initiative create` - name, description, owner, target date, status (**Mutation** `initiativeCreate`)
- [x] `initiative update <id>` - edit any field (**Mutation** `initiativeUpdate`)
- [x] `initiative archive <id>` / `unarchive <id>` (**Mutation** `initiativeArchive`, `initiativeUnarchive`)
- [x] `initiative delete <id>` (**Mutation** `initiativeDelete`)
- [x] `initiative set-lead <id>` - change lead and owning team (**Mutation** `initiativeLeadTeamUpdate`)

### Initiative to project links
- [x] `initiative add-project <id> <project>` (**Mutation** `initiativeToProjectCreate`)
- [x] `initiative remove-project <id> <project>` (**Mutation** `initiativeToProjectDelete`)
- [x] `initiative reorder-project` - change sort order (**Mutation** `initiativeToProjectUpdate`)
- [x] Query linked projects (**Query** `initiativeToProjects`, `initiativeToProject`)

### Initiative labels and relations
- [x] `initiative label add/remove <id>` (**Mutation** `initiativeAddLabel`, `initiativeRemoveLabel`)
- [x] `initiative relate <id> <other>` - link initiatives (**Mutation** `initiativeRelationCreate`, `initiativeRelationUpdate`, `initiativeRelationDelete`; **Query** `initiativeRelations`)

### Initiative updates (status posts)
- [x] `initiative update-post list <id>` - read status updates (**Query** `initiativeUpdates`, `initiativeUpdate`)
- [x] `initiative update-post create <id>` - post a status update with health (**Mutation** `initiativeUpdateCreate`)
- [x] `initiative update-post edit / archive / unarchive` (**Mutation** `initiativeUpdateUpdate`, `initiativeUpdateArchive`, `initiativeUpdateUnarchive`)
- [x] `initiative update-reminder <id>` - request an update reminder (**Mutation** `createInitiativeUpdateReminder`)

> Naming note: Linear uses `initiativeUpdate` for editing the initiative itself and
> `initiativeUpdateCreate` for posting a status update. The command names above keep these
> apart with `update` versus `update-post`.

---

## Tier 1 - Projects

Projects are read only today (`list`, `get`). Every write path is missing.

### Project CRUD
- [x] `project create` - name, description, team(s), lead, dates, status (**Mutation** `projectCreate`)
- [x] `project update <id>` - edit name, description, state, health, lead, dates, members (**Mutation** `projectUpdate`)
- [x] `project delete <id>` (**Mutation** `projectDelete`)
- [x] `project unarchive <id>` (**Mutation** `projectUnarchive`)

> Note: `projectArchive` is deprecated. Archiving now happens through `projectUpdate` (set
> state) or `projectDelete`, which the commands above would cover.

### Project members
- [x] `project members <id>` - list members (available via **Query** `project.members`)
- [x] `project add-member` / `remove-member` - set `memberIds` through `projectUpdate` (**Mutation** `projectUpdate`)

### Project milestones
- [x] `project milestone list <project>` (**Query** `projectMilestones`, `projectMilestone`)
- [x] `project milestone create <project>` - name, description, target date (**Mutation** `projectMilestoneCreate`)
- [x] `project milestone update <id>` (**Mutation** `projectMilestoneUpdate`)
- [x] `project milestone move <id>` - reorder or move between projects (**Mutation** `projectMilestoneMove`)
- [x] `project milestone delete <id>` (**Mutation** `projectMilestoneDelete`)
- [x] Assign an issue to a milestone (add `--milestone` to `issue create` / `issue update`)

### Project updates (status posts)
- [x] `project update-post list <project>` (**Query** `projectUpdates`, `projectUpdate`)
- [x] `project update-post create <project>` - body plus health (**Mutation** `projectUpdateCreate`)
- [x] `project update-post edit / archive / unarchive` (**Mutation** `projectUpdateUpdate`, `projectUpdateArchive`, `projectUpdateUnarchive`)
- [x] `project update-reminder <project>` (**Mutation** `createProjectUpdateReminder`)

> Note: `projectUpdateDelete` is deprecated; archive replaces delete here.

### Project labels
- [x] `project label list` (**Query** `projectLabels`, `projectLabel`)
- [x] `project label create / update / delete` (**Mutation** `projectLabelCreate`, `projectLabelUpdate`, `projectLabelDelete`)
- [x] `project label retire / restore` (**Mutation** `projectLabelRetire`, `projectLabelRestore`)
- [x] `project label add / remove <project>` (**Mutation** `projectAddLabel`, `projectRemoveLabel`)

### Project relations and status
- [x] `project relate <id> <other>` - dependency links (**Mutation** `projectRelationCreate`, `projectRelationUpdate`, `projectRelationDelete`; **Query** `projectRelations`)
- [x] `project status list` - org level project statuses (**Query** `projectStatuses`)
- [x] `project status create / update / archive / unarchive` (**Mutation** `projectStatusCreate`, `projectStatusUpdate`, `projectStatusArchive`, `projectStatusUnarchive`)
- [x] `project reassign-status` - bulk move projects to a new status (**Mutation** `projectReassignStatus`)

---

## Tier 2 - Cycles

No cycle commands exist. Cycle data appears only inside `issue get`.

- [x] `cycle list [--team]` (**Query** `cycles`, `cycle`)
- [x] `cycle get <id>` - detail with progress and scope history
- [x] `cycle create` (**Mutation** `cycleCreate`)
- [x] `cycle update <id>` (**Mutation** `cycleUpdate`)
- [x] `cycle archive <id>` (**Mutation** `cycleArchive`)
- [x] `cycle shift <team>` - shift all future cycles (**Mutation** `cycleShiftAll`)
- [x] `cycle start-now <team>` - start the upcoming cycle today (**Mutation** `cycleStartUpcomingCycleToday`)
- [x] Assign an issue to a cycle (add `--cycle` to `issue create` / `issue update`)

---

## Tier 2 - Labels (issue labels)

No label management. Labels are read only inside `issue get`.

- [x] `label list [--team]` (**Query** `issueLabels`, `issueLabel`)
- [x] `label create` - name, color, description, parent group (**Mutation** `issueLabelCreate`)
- [x] `label update <id>` (**Mutation** `issueLabelUpdate`)
- [x] `label delete <id>` (**Mutation** `issueLabelDelete`)
- [x] `label retire / restore <id>` (**Mutation** `issueLabelRetire`, `issueLabelRestore`)
- [x] Set labels when creating or editing an issue (add `--label` to `issue create` / `issue update`, or `issueAddLabel` / `issueRemoveLabel`)

---

## Tier 2 - Workflow states

States are read only (inside `team get`).

- [x] `state list <team>` (**Query** `workflowStates`, `workflowState`)
- [x] `state create <team>` - name, type, color, position (**Mutation** `workflowStateCreate`)
- [x] `state update <id>` (**Mutation** `workflowStateUpdate`)
- [x] `state archive <id>` (**Mutation** `workflowStateArchive`)

---

## Tier 2 - Issue lifecycle gaps

`issue create`, `update`, `assign`, and `link` exist. These paths are still missing.

### Lifecycle
- [x] `issue archive <id>` / `unarchive <id>` (**Mutation** `issueArchive`, `issueUnarchive`) - README lists archive as "coming soon"
- [x] `issue delete <id>` (**Mutation** `issueDelete`)
- [x] `issue subscribe / unsubscribe <id>` (**Mutation** `issueSubscribe`, `issueUnsubscribe`)
- [x] `issue remind <id>` - set a reminder (**Mutation** `issueReminder`)
- [x] `issue share <id>` / `unshare` - public share link (**Mutation** `issueShare`, `issueUnshare`)

### Richer create and update
`issue create` accepts only title, description, team, priority, and assign-me.
`issue update` accepts only title, description, assignee, state, priority, due-date,
project. The following inputs are supported by the API but not exposed:

- [x] `--label` - set labels (**Mutation** `issueAddLabel`, `issueRemoveLabel`, or `labelIds` in create/update)
- [x] `--cycle` - assign to a cycle
- [x] `--estimate` - set story points
- [x] `--parent` - set parent issue on create (currently only via `issue link`)
- [x] `--milestone` - assign to a project milestone
- [x] `--project` on create, `--assignee` on create, `--state` on create, `--due-date` on create

### Bulk operations
- [x] `issue batch-create` (**Mutation** `issueBatchCreate`)
- [x] `issue batch-update` - update many issues at once (**Mutation** `issueBatchUpdate`)

---

## Tier 2 - Comments

`comment list` and `comment create` exist. Note that a `commentUpdate` operation is already
defined in `operations/comments.graphql` but no command is wired to it.

- [x] `comment edit <id>` - wire up the existing `commentUpdate` operation (**Mutation** `commentUpdate`)
- [x] `comment delete <id>` (**Mutation** `commentDelete`)
- [x] `comment resolve / unresolve <id>` (**Mutation** `commentResolve`, `commentUnresolve`)
- [x] `comment reply` - threaded reply via `--parent` on create (input already supports `parentId`)
- [x] React to a comment or issue (**Mutation** `reactionCreate`, `reactionDelete`)

---

## Tier 2 - Documents

Documents appear only inside `project get`. No standalone commands.

- [x] `document list` (**Query** `documents`, `document`)
- [x] `document search <term>` (**Query** `searchDocuments`)
- [x] `document get <id>` - content and history (**Query** `documentContentHistory`)
- [x] `document create` - title, content, project or initiative (**Mutation** `documentCreate`)
- [x] `document update <id>` (**Mutation** `documentUpdate`)
- [x] `document delete / unarchive <id>` (**Mutation** `documentDelete`, `documentUnarchive`)

---

## Tier 3 - Teams

`team list`, `get`, `members` exist (all read only).

- [x] `team create` (**Mutation** `teamCreate`)
- [x] `team update <key>` (**Mutation** `teamUpdate`)
- [x] `team delete <key>` / `unarchive` (**Mutation** `teamDelete`, `teamUnarchive`)
- [x] `team add-member / remove-member / set-role` (**Mutation** `teamMembershipCreate`, `teamMembershipDelete`, `teamMembershipUpdate`)

---

## Tier 3 - Users and organization

`user list`, `get`, `me` exist (all read only).

- [x] `user update` - edit own profile (**Mutation** `userUpdate`)
- [x] `user set-role <id>` - admin, member, guest (**Mutation** `userChangeRole`)
- [x] `user suspend / unsuspend <id>` (**Mutation** `userSuspend`, `userUnsuspend`)
- [x] `org get` - organization detail (**Query** `organization`)
- [x] `org update` (**Mutation** `organizationUpdate`)
- [x] `org invite list / create / delete / resend` (**Mutation** `organizationInviteCreate`, `organizationInviteDelete`, `resendOrganizationInvite`; **Query** `organizationInvites`)

---

## Tier 3 - Favorites, views, notifications, webhooks, templates

### Favorites
- [x] `favorite list` (**Query** `favorites`, `favorite`)
- [x] `favorite add / remove` - issues, projects, cycles, and more (**Mutation** `favoriteCreate`, `favoriteUpdate`, `favoriteDelete`)

### Custom views
- [x] `view list` (**Query** `customViews`, `customView`)
- [x] `view create / update / delete` (**Mutation** `customViewCreate`, `customViewUpdate`, `customViewDelete`)

### Notifications
- [x] `notification list` (**Query** `notifications`, `notification`, `notificationsUnreadCount`)
- [x] `notification read-all / unread-all` (**Mutation** `notificationMarkReadAll`, `notificationMarkUnreadAll`)
- [x] `notification archive / snooze` (**Mutation** `notificationArchive`, `notificationArchiveAll`, `notificationSnoozeAll`)
- [x] Notification subscription management (**Mutation** `notificationSubscriptionCreate`, `notificationSubscriptionUpdate`)

### Webhooks
- [x] `webhook list / get` (**Query** `webhooks`, `webhook`)
- [x] `webhook create / update / delete` (**Mutation** `webhookCreate`, `webhookUpdate`, `webhookDelete`)
- [x] `webhook rotate-secret` (**Mutation** `webhookRotateSecret`)

### Templates
- [x] `template list` - issue and project templates (**Query** `templates`, `template`)
- [x] `template create / update / delete` (**Mutation** `templateCreate`, `templateUpdate`, `templateDelete`)
- [x] Apply a template when creating an issue or project

---

## Tier 4 - Advanced domains (in scope)

Complete API areas the CLI does not touch, now enumerated operation by operation.

### Releases
Release-tracking domain (~30 operations). Suggested `release` command group.
- [x] `release list / get / search` (**Query** `releases`, `release`, `releaseSearch`)
- [x] `release create / update / delete` (**Mutation** `releaseCreate`, `releaseUpdate`, `releaseDelete`)
- [x] `release archive / unarchive / complete` (**Mutation** `releaseArchive`, `releaseUnarchive`, `releaseComplete`)
- [x] `release sync` and pipeline-driven updates (**Mutation** `releaseSync`, `releaseUpdateByPipeline`)
- [x] `release pipeline list / create / update / delete / archive / unarchive` (**Query** `releasePipelines`, `releasePipeline`; **Mutation** `releasePipelineCreate`, `releasePipelineUpdate`, `releasePipelineDelete`, `releasePipelineArchive`, `releasePipelineUnarchive`)
- [x] `release stage list / create / update / archive / unarchive` (**Query** `releaseStages`, `releaseStage`; **Mutation** `releaseStageCreate`, `releaseStageUpdate`, `releaseStageArchive`, `releaseStageUnarchive`)
- [x] `release note list / create / update / delete` (**Query** `releaseNotes`, `releaseNote`; **Mutation** `releaseNoteCreate`, `releaseNoteUpdate`, `releaseNoteDelete`)
- [x] Link issues to releases (**Query** `issueToReleases`, `issueToRelease`; **Mutation** `issueToReleaseCreate`, `issueToReleaseDelete`, `issueToReleaseDeleteByIssueAndRelease`)

> Access-key variants (`releaseCompleteByAccessKey`, `releaseSyncByAccessKey`, `latestReleaseByAccessKey`, `recentReleasesByAccessKey`, `releasePipelineByAccessKey`) support unauthenticated pipeline reads and writes. Wire only if a CI use case needs them.

### Customers / CRM
Suggested `customer` command group.
- [x] `customer list / get` (**Query** `customers`, `customer`)
- [x] `customer create / update / delete / merge / upsert / unsync` (**Mutation** `customerCreate`, `customerUpdate`, `customerDelete`, `customerMerge`, `customerUpsert`, `customerUnsync`)
- [x] `customer need list / get / create / update / delete / archive / unarchive` (**Query** `customerNeeds`, `customerNeed`; **Mutation** `customerNeedCreate`, `customerNeedUpdate`, `customerNeedDelete`, `customerNeedArchive`, `customerNeedUnarchive`, `customerNeedCreateFromAttachment`)
- [x] `customer status list / create / update / delete` (**Query** `customerStatuses`, `customerStatus`; **Mutation** `customerStatusCreate`, `customerStatusUpdate`, `customerStatusDelete`)
- [x] `customer tier list / create / update / delete` (**Query** `customerTiers`, `customerTier`; **Mutation** `customerTierCreate`, `customerTierUpdate`, `customerTierDelete`)

### Reactions and emoji
- [x] React and unreact on issues and comments (**Mutation** `reactionCreate`, `reactionDelete`)
- [x] Custom org emoji list / create / delete (**Query** `emojis`, `emoji`; **Mutation** `emojiCreate`, `emojiDelete`)

### Triage responsibilities
- [x] `triage list / get / create / update / delete` (**Query** `triageResponsibilities`, `triageResponsibility`; **Mutation** `triageResponsibilityCreate`, `triageResponsibilityUpdate`, `triageResponsibilityDelete`)

### Git automation
- [x] Git automation states: create / update / delete (**Mutation** `gitAutomationStateCreate`, `gitAutomationStateUpdate`, `gitAutomationStateDelete`)
- [x] Git automation target branches: create / update / delete (**Mutation** `gitAutomationTargetBranchCreate`, `gitAutomationTargetBranchUpdate`, `gitAutomationTargetBranchDelete`)

### Time schedules / on-call
- [x] `schedule list / get` (**Query** `timeSchedules`, `timeSchedule`)
- [x] `schedule create / update / delete` (**Mutation** `timeScheduleCreate`, `timeScheduleUpdate`, `timeScheduleDelete`)
- [x] `schedule upsert-external / refresh` (**Mutation** `timeScheduleUpsertExternal`, `timeScheduleRefreshIntegrationSchedule`)

### OAuth applications (developer API management)
- [x] `oauth-app list / get` (**Query** `oauthApplications`, `oauthApplication`)
- [x] `oauth-app create / update / archive` (**Mutation** `oauthApplicationCreate`, `oauthApplicationUpdate`, `oauthApplicationArchive`)
- [x] `oauth-app rotate-secret / rotate-webhook-secret` (**Mutation** `oauthApplicationRotateSecret`, `oauthApplicationRotateWebhookSecret`)

### Agents API (lowest priority)
> For building Linear AI-agent integrations, not general CRUD. In scope, but build this last and confirm the use case first.
- [x] Agent sessions (**Query** `agentSessions`, `agentSession`, `agentSessionSandbox`; **Mutation** `agentSessionCreate`, `agentSessionCreateOnIssue`, `agentSessionCreateOnComment`, `agentSessionUpdate`, `agentSessionUpdateExternalUrl`)
- [x] Agent activities (**Query** `agentActivities`, `agentActivity`; **Mutation** `agentActivityCreate`, `agentActivityCreatePrompt`, `agentActivitySendQueued`, `agentActivityDeleteQueued`)
- [x] Agent skills (**Query** `agentSkills`, `agentSkill`; **Mutation** `agentSkillCreate`, `agentSkillUpdate`, `agentSkillDelete`)

### Attachment link helpers
- [x] One `attachment link --type <provider>` command backed by the typed link mutations: `attachmentLinkURL`, `attachmentLinkSlack`, `attachmentLinkDiscord`, `attachmentLinkFront`, `attachmentLinkIntercom`, `attachmentLinkZendesk`, `attachmentLinkJiraIssue`, `attachmentLinkGitHubIssue`, `attachmentLinkGitHubPR`, `attachmentLinkGitLabMR`, `attachmentLinkSalesforce`
- [x] `attachment sync-to-slack` (**Mutation** `attachmentSyncToSlack`)

---

## Tier 5 - Additional operations (in scope)

Items surfaced during the full API audit that the earlier tiers missed.

### Audit log
- [x] `audit list` - workspace change history (**Query** `auditEntries`, `auditEntryTypes`)

### Global search
- [x] `search projects <term>` (**Query** `searchProjects`)
- [x] `search semantic <term>` - AI semantic search across the workspace (**Query** `semanticSearch`)

### Diagnostics
- [x] `rate-limit` - show the current API rate-limit budget (**Query** `rateLimitStatus`)

### External links (generic)
- [x] `link add / update / remove` on any entity, including initiatives and documents (**Query** `entityExternalLink`; **Mutation** `entityExternalLinkCreate`, `entityExternalLinkUpdate`, `entityExternalLinkDelete`)

### View preferences
- [x] Per-view display config for saved views (**Mutation** `viewPreferencesCreate`, `viewPreferencesUpdate`, `viewPreferencesDelete`)

### User settings
- [x] `user settings update` - update your own settings (**Mutation** `userSettingsUpdate`)

### Create and update helpers (internals, not standalone commands)
- [x] Enrich `issue create` / `update` and branch tooling (**Query** `issuePriorityValues`, `issueFilterSuggestion`, `issueVcsBranchSearch`, `issueRepositorySuggestions`)

### Miscellaneous
- [x] `issue relate` update support (**Mutation** `issueRelationUpdate`)
- [x] SLA configurations (**Query** `slaConfigurations`)
- [x] External and guest users (**Query** `externalUsers`, `externalUser`)

---

## Excluded from scope

Confirmed out of scope. These are login and OAuth handshakes, telemetry, device and account
internals, org administration, and one-time migrations that belong in the Linear web app
rather than a personal CLI.

- **Integration connectors (~56 mutations plus read and settings queries)**: `integrationSlack*`,
  `integrationGithub*`, `integrationJira*`, `integrationMicrosoftTeams*`, `integrationGitlab*`,
  `integrationSalesforce*`, `integrationZendesk`, `integrationIntercom*`, `integrationSentryConnect`,
  `integrationOpsgenie*`, `integrationPagerDuty*`, `integrationFigma`, `integrationDiscord`,
  `integrationGong`, `integrationGoogleSheets`, `integrationGoogleCalendarPersonalConnect`,
  `integrationLaunchDarkly*`, `integrationMcpServer*`, `integrationRequest`, `integrationArchive`,
  `integrationDelete`, `integrationUpdate`, `integrationTemplate*`, `integrationsSettings*`,
  `updateIntegrationSlackScopes`, `airbyteIntegrationConnect`, `jiraIntegrationConnect`; plus
  queries `integration(s)`, `integrationHasScopes`, `integrationTemplate(s)`, `archivedIntegrations`,
  `microsoftTeamsChannels`
- **Auth and sessions**: `googleUserAccountAuth`, `samlTokenUserAccountAuth`, `passkeyLoginStart`,
  `passkeyLoginFinish`, `emailTokenUserAccountAuth`, `emailUserAccountAuthChallenge`, `logout`,
  `logoutSession`, `logoutAllSessions`, `logoutOtherSessions`, `userRevokeSession`,
  `userRevokeAllSessions`, `userSessions`, `authenticationSessions`, `ssoUrlFromEmail`,
  `verifyGitHubEnterpriseServerInstallation`
- **Telemetry, onboarding, and exports**: `trackAnonymousEvent`, `createCsvExportReport`,
  `createOrganizationFromOnboarding`, `joinOrganizationFromOnboarding`, `leaveOrganization`,
  `contactCreate`, `contactSalesCreate`
- **Upload internals** (already used under the hood by `attachment upload`): `fileUpload`,
  `fileUploadDangerouslyDelete`, `imageUploadFromUrl`, `importFileUpload`, `refreshGoogleSheetsData`
- **Push notifications and device**: `pushSubscriptionCreate`, `pushSubscriptionDelete`,
  `pushSubscriptionTest`
- **Email intake addresses**: `emailIntakeAddress*`
- **Org administration**: `organizationDomain*`, `organizationDelete`, `organizationCancelDelete`,
  `organizationDeleteChallenge`, `organizationStartTrialForPlan`, `resendOrganizationInviteByEmail`,
  `userFlagUpdate`, `userSettingsFlagsReset`, `userDiscordConnect`, `userExternalUserDisconnect`,
  `userUnlinkFromIdentityProvider`
- **Issue and project migration** (one-time bulk imports, done via the web importers): `issueImportCreateAsana`,
  `issueImportCreateJira`, `issueImportCreateGithub`, `issueImportCreateCSVJira`, `issueImportCreateClubhouse`,
  `issueImportCreateLinearV2`, `issueImportUpdate`, `issueImportProcess`, `issueImportDelete`,
  `issueImportCheck*`, `issueExternalSyncDisable`, `issueDescriptionUpdateFromFront`,
  `projectExternalSyncDisable`, `projectCreateSlackChannel`
- **Roadmaps** - deprecated by Linear in favor of Initiatives; `roadmapToProject*` and the removed
  top-level roadmap operations stay out

---

## Appendix: coverage math

- Live API: 353 mutations + 153 queries = 506 operations
- Already implemented: ~15
- In scope (Tiers 1-5): ~330 operations
- Excluded (see the section above): ~140 operations, dominated by ~60 integration connectors
- The small remainder (~40) are read, detail, and suggestion queries that pair with in-scope
  domains (for example `teamMemberships`, `projectRelations`, `notificationSubscriptions`,
  `userSettings`). They get pulled in alongside their parent commands. A few internal
  endpoints (`applicationInfo`, `fetchData`) stay out.

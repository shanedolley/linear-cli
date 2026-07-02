package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
)

// uuidPattern matches a canonical (hyphenated) UUID, case-insensitively.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID reports whether s looks like a canonical UUID. It is used by the
// resolvers below to short-circuit lookups: when a caller already passed an
// ID (rather than a human-friendly reference like a team key, email, or
// name), we skip the API round-trip entirely.
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// workflowStateInfo is a minimal, resolver-local view of a team's workflow
// state, used for case-insensitive name matching and building "not found"
// error messages.
type workflowStateInfo struct {
	id   string
	name string
}

// projectStatusInfo is a minimal, resolver-local view of a workspace-wide
// ProjectStatus, used for case-insensitive name matching and building "not
// found" error messages in resolveProjectStatus.
type projectStatusInfo struct {
	id   string
	name string
}

// milestoneInfo is a minimal, resolver-local view of a ProjectMilestone,
// used for case-insensitive name matching and building "not found"/
// "ambiguous" error messages in resolveMilestone.
type milestoneInfo struct {
	id   string
	name string
}

// ResolverCache memoizes entity resolution (team/user/project/state) for the
// lifetime of a single command invocation. It is intentionally simple: a
// fresh cache is created per invocation via newResolverCache, so entries
// never need to be invalidated or expired.
type ResolverCache struct {
	teams            map[string]string
	users            map[string]string
	projects         map[string]string
	initiatives      map[string]string
	states           map[string][]workflowStateInfo
	labels           map[string]string
	initiativeLabels map[string]string
	projectLabels    map[string]string

	// projectStatuses caches the workspace's ProjectStatus catalog, fetched
	// at most once per invocation. It is workspace-wide (not keyed by
	// anything), unlike states which are per-team. projectStatusesLoaded
	// distinguishes "not fetched yet" from "fetched, workspace has none".
	projectStatuses       []projectStatusInfo
	projectStatusesLoaded bool

	// milestones caches each project's milestone list, keyed by project ID.
	milestones map[string][]milestoneInfo
}

// newResolverCache creates an empty ResolverCache. Call this once per
// command invocation and thread it through any resolve* calls so repeated
// references to the same team/user/project/initiative/state/label only hit
// the API once.
func newResolverCache() *ResolverCache {
	return &ResolverCache{
		teams:            make(map[string]string),
		users:            make(map[string]string),
		projects:         make(map[string]string),
		initiatives:      make(map[string]string),
		states:           make(map[string][]workflowStateInfo),
		labels:           make(map[string]string),
		initiativeLabels: make(map[string]string),
		projectLabels:    make(map[string]string),
		milestones:       make(map[string][]milestoneInfo),
	}
}

// resolveTeam resolves a team key (e.g. "ENG") or UUID to a team ID.
//
// Team keys are matched case-insensitively even though Linear stores them
// uppercase: the input is uppercased before the primary lookup, and a
// secondary scan over ListTeams is used as a fallback in case the direct
// lookup fails.
func resolveTeam(ctx context.Context, client graphql.Client, cache *ResolverCache, keyOrID string) (string, error) {
	if isUUID(keyOrID) {
		return keyOrID, nil
	}

	cacheKey := strings.ToUpper(keyOrID)
	if id, ok := cache.teams[cacheKey]; ok {
		return id, nil
	}

	// Fast path: Linear's team lookup accepts a team key directly, so try
	// the normalized (uppercased) key first without listing every team.
	if resp, err := api.GetTeam(ctx, client, cacheKey); err == nil && resp.Team != nil {
		id := resp.Team.TeamDetailFields.Id
		cache.teams[cacheKey] = id
		return id, nil
	}

	// Fallback: scan all teams for a case-insensitive key match.
	limit := 250
	listResp, err := api.ListTeams(ctx, client, &limit, nil, nil)
	if err != nil {
		return "", fmt.Errorf("Team not found: %s", keyOrID)
	}

	for _, node := range listResp.Teams.Nodes {
		if strings.EqualFold(node.TeamListFields.Key, keyOrID) {
			id := node.TeamListFields.Id
			cache.teams[cacheKey] = id
			return id, nil
		}
	}

	return "", fmt.Errorf("Team not found: %s", keyOrID)
}

// resolveUser resolves a user reference to a user ID. ref may be:
//   - a UUID, returned as-is
//   - the literal "me", resolved to the authenticated viewer
//   - an email address or display name, matched exactly (case-sensitively,
//     mirroring Linear's own comparator) via the same UserFilter pattern
//     used elsewhere in this package
//
// A reference matching more than one user returns an error listing the
// candidates; a reference matching no user returns a "not found" error.
func resolveUser(ctx context.Context, client graphql.Client, cache *ResolverCache, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}

	if strings.EqualFold(ref, "me") {
		cacheKey := "me"
		if id, ok := cache.users[cacheKey]; ok {
			return id, nil
		}
		resp, err := api.GetViewer(ctx, client)
		if err != nil {
			return "", fmt.Errorf("failed to resolve current user: %w", err)
		}
		id := resp.Viewer.UserDetailFields.Id
		cache.users[cacheKey] = id
		return id, nil
	}

	cacheKey := strings.ToLower(ref)
	if id, ok := cache.users[cacheKey]; ok {
		return id, nil
	}

	filter := &api.UserFilter{
		Or: []*api.UserFilter{
			{Email: &api.StringComparator{Eq: &ref}},
			{Name: &api.StringComparator{Eq: &ref}},
		},
	}

	resp, err := api.GetUserByEmail(ctx, client, filter)
	if err != nil {
		return "", fmt.Errorf("failed to find user '%s': %w", ref, err)
	}

	nodes := resp.Users.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("User not found: %s", ref)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s <%s>", n.UserDetailFields.Name, n.UserDetailFields.Email))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", ref, strings.Join(candidates, ", "))
	}

	id := nodes[0].UserDetailFields.Id
	cache.users[cacheKey] = id
	return id, nil
}

// resolveProject resolves a project name or UUID to a project ID. Name
// matching is case-insensitive. A name matching more than one project
// returns an error listing the candidates (name and id); a name matching no
// project returns a "not found" error.
func resolveProject(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.projects[cacheKey]; ok {
		return id, nil
	}

	filter := &api.ProjectFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListProjects(ctx, client, filter, &limit, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find project '%s': %w", nameOrID, err)
	}

	nodes := resp.Projects.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Project not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.ProjectListFields.Name, n.ProjectListFields.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].ProjectListFields.Id
	cache.projects[cacheKey] = id
	return id, nil
}

// resolveInitiative resolves an initiative name or UUID to an initiative ID.
// Name matching is case-insensitive. A name matching more than one initiative
// returns an error listing the candidates (name and id); a name matching no
// initiative returns a "not found" error.
func resolveInitiative(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.initiatives[cacheKey]; ok {
		return id, nil
	}

	filter := &api.InitiativeFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListInitiatives(ctx, client, filter, &limit, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find initiative '%s': %w", nameOrID, err)
	}

	nodes := resp.Initiatives.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Initiative not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.InitiativeListFields.Name, n.InitiativeListFields.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].InitiativeListFields.Id
	cache.initiatives[cacheKey] = id
	return id, nil
}

// resolveLabel resolves an issue label name or UUID to an IssueLabel ID.
// Name matching is case-insensitive. A name matching more than one label
// returns an error listing the candidates (name and id); a name matching no
// label returns a "not found" error.
//
// This resolves against the IssueLabel catalog (issues/teams). It is not
// used by the `initiative label` commands: initiative labels are a
// separate InitiativeLabel catalog with its own id space (confirmed live -
// initiativeAddLabel/initiativeRemoveLabel reject an IssueLabel id). See
// resolveInitiativeLabel for that catalog.
func resolveLabel(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.labels[cacheKey]; ok {
		return id, nil
	}

	filter := &api.IssueLabelFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListIssueLabels(ctx, client, filter, &limit)
	if err != nil {
		return "", fmt.Errorf("failed to find label '%s': %w", nameOrID, err)
	}

	nodes := resp.IssueLabels.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Label not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.Name, n.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].Id
	cache.labels[cacheKey] = id
	return id, nil
}

// resolveInitiativeLabel resolves an initiative label name or UUID to an
// InitiativeLabel ID. Name matching is case-insensitive. A name matching
// more than one label returns an error listing the candidates (name and
// id); a name matching no label returns a "not found" error.
//
// Initiative labels are a distinct catalog from IssueLabel (see
// resolveLabel), confirmed live against the Linear API. The whole feature
// is workspace-gated; on a workspace where it's disabled, the underlying
// ListInitiativeLabels call fails with a clear "Feature ... is not enabled"
// error from Linear, which is surfaced as-is.
func resolveInitiativeLabel(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.initiativeLabels[cacheKey]; ok {
		return id, nil
	}

	filter := &api.InitiativeLabelFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListInitiativeLabels(ctx, client, filter, &limit)
	if err != nil {
		return "", fmt.Errorf("failed to find initiative label '%s': %w", nameOrID, err)
	}

	nodes := resp.InitiativeLabels.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Initiative label not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.Name, n.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].Id
	cache.initiativeLabels[cacheKey] = id
	return id, nil
}

// resolveProjectLabel resolves a project label name or UUID to a
// ProjectLabel ID. Name matching is case-insensitive. A name matching more
// than one label returns an error listing the candidates (name and id); a
// name matching no label returns a "not found" error.
//
// ProjectLabel is a distinct, workspace-wide catalog from both IssueLabel
// (see resolveLabel) and InitiativeLabel (see resolveInitiativeLabel) -
// confirmed against schema.graphql, which has no cross-reference between
// the three label types.
func resolveProjectLabel(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.projectLabels[cacheKey]; ok {
		return id, nil
	}

	filter := &api.ProjectLabelFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListProjectLabels(ctx, client, filter, &limit)
	if err != nil {
		return "", fmt.Errorf("failed to find project label '%s': %w", nameOrID, err)
	}

	nodes := resp.ProjectLabels.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Project label not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.Name, n.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].Id
	cache.projectLabels[cacheKey] = id
	return id, nil
}

// resolveWorkflowState resolves a workflow state name or UUID to a state ID
// within the given team. teamKeyOrID may be a team key or a team ID (both
// are accepted by Linear's team lookup); nameOrID is matched case-
// insensitively against the team's workflow states. A name matching no
// state returns an error listing the team's valid state names.
func resolveWorkflowState(ctx context.Context, client graphql.Client, cache *ResolverCache, teamKeyOrID, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	states, ok := cache.states[teamKeyOrID]
	if !ok {
		resp, err := api.GetTeamStates(ctx, client, teamKeyOrID)
		if err != nil {
			return "", fmt.Errorf("failed to get workflow states for team '%s': %w", teamKeyOrID, err)
		}
		if resp.Team == nil || resp.Team.States == nil {
			return "", fmt.Errorf("no workflow states found for team '%s'", teamKeyOrID)
		}

		states = make([]workflowStateInfo, 0, len(resp.Team.States.Nodes))
		for _, s := range resp.Team.States.Nodes {
			states = append(states, workflowStateInfo{id: s.Id, name: s.Name})
		}
		cache.states[teamKeyOrID] = states
	}

	for _, s := range states {
		if strings.EqualFold(s.name, nameOrID) {
			return s.id, nil
		}
	}

	names := make([]string, 0, len(states))
	for _, s := range states {
		names = append(names, s.name)
	}
	return "", fmt.Errorf("State '%s' not found in team '%s'. Available states: %s", nameOrID, teamKeyOrID, strings.Join(names, ", "))
}

// resolveProjectStatus resolves a project status name or UUID to a
// ProjectStatus ID. ProjectStatus is a workspace-wide catalog (unlike
// workflow states, which are per-team), so the fetched list is cached
// unscoped on the ResolverCache. Name matching is case-insensitive. A name
// matching no status returns an error listing the workspace's valid status
// names.
func resolveProjectStatus(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	if !cache.projectStatusesLoaded {
		limit := 250
		resp, err := api.ListProjectStatuses(ctx, client, &limit)
		if err != nil {
			return "", fmt.Errorf("failed to list project statuses: %w", err)
		}

		statuses := []projectStatusInfo{}
		if resp.ProjectStatuses != nil {
			for _, s := range resp.ProjectStatuses.Nodes {
				statuses = append(statuses, projectStatusInfo{id: s.ProjectStatusFields.Id, name: s.ProjectStatusFields.Name})
			}
		}
		cache.projectStatuses = statuses
		cache.projectStatusesLoaded = true
	}

	for _, s := range cache.projectStatuses {
		if strings.EqualFold(s.name, nameOrID) {
			return s.id, nil
		}
	}

	names := make([]string, 0, len(cache.projectStatuses))
	for _, s := range cache.projectStatuses {
		names = append(names, s.name)
	}
	return "", fmt.Errorf("Project status '%s' not found. Valid values: %s", nameOrID, strings.Join(names, ", "))
}

// resolveMilestone resolves a project milestone name or UUID to a
// ProjectMilestone ID, scoped to the given project. Name matching is
// case-insensitive. A name matching more than one milestone within the
// project returns an error listing the candidates (name and id); a name
// matching none returns a "not found" error listing the project's milestone
// names.
func resolveMilestone(ctx context.Context, client graphql.Client, cache *ResolverCache, projectID, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	milestones, ok := cache.milestones[projectID]
	if !ok {
		limit := 250
		resp, err := api.ListProjectMilestones(ctx, client, projectID, &limit)
		if err != nil {
			return "", fmt.Errorf("failed to list milestones for project: %w", err)
		}
		if resp.Project == nil || resp.Project.ProjectMilestones == nil {
			return "", fmt.Errorf("project not found or has no milestones")
		}

		milestones = make([]milestoneInfo, 0, len(resp.Project.ProjectMilestones.Nodes))
		for _, m := range resp.Project.ProjectMilestones.Nodes {
			milestones = append(milestones, milestoneInfo{id: m.ProjectMilestoneFields.Id, name: m.ProjectMilestoneFields.Name})
		}
		cache.milestones[projectID] = milestones
	}

	var matches []milestoneInfo
	for _, m := range milestones {
		if strings.EqualFold(m.name, nameOrID) {
			matches = append(matches, m)
		}
	}

	if len(matches) == 0 {
		names := make([]string, 0, len(milestones))
		for _, m := range milestones {
			names = append(names, m.name)
		}
		return "", fmt.Errorf("Milestone '%s' not found in project. Available milestones: %s", nameOrID, strings.Join(names, ", "))
	}
	if len(matches) > 1 {
		candidates := make([]string, 0, len(matches))
		for _, m := range matches {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", m.name, m.id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	return matches[0].id, nil
}

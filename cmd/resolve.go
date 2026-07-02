package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

	// cycles caches resolved cycle IDs, keyed by "teamID:lower(ref)". Cycles
	// are scoped to a team and referenced by number or name, so the team ID is
	// part of the key.
	cycles map[string]string

	// templates caches resolved template IDs, keyed by "type:lower(name)".
	// Templates are matched by name within a type (issue or project), so the
	// type is part of the key.
	templates map[string]string

	// customers caches resolved customer IDs, keyed by lower(name).
	customers map[string]string

	// customerStatuses and customerTiers cache the workspace's customer status
	// and tier catalogs, keyed by lower(name); each is fetched at most once per
	// invocation. The *Loaded bools distinguish "not fetched yet" from "fetched,
	// workspace has none".
	customerStatuses       map[string]string
	customerStatusesLoaded bool
	customerTiers          map[string]string
	customerTiersLoaded    bool

	// timeSchedules caches the workspace's time schedule catalog, keyed by
	// lower(name); fetched at most once per invocation. timeSchedulesLoaded
	// distinguishes "not fetched yet" from "fetched, workspace has none".
	timeSchedules       map[string]string
	timeSchedulesLoaded bool
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
		cycles:           make(map[string]string),
		templates:        make(map[string]string),
		customers:        make(map[string]string),
		customerStatuses: make(map[string]string),
		customerTiers:    make(map[string]string),
		timeSchedules:    make(map[string]string),
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
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.LabelListFields.Name, n.LabelListFields.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].LabelListFields.Id
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

// resolveTeamMembership resolves a (team, user) pair to the user's team
// membership ID, needed to update or remove a membership. It resolves the team
// key/ID and user reference, then scans the team's memberships for that user,
// paging through all memberships (the connection defaults to 50 per page, so a
// single page would miss members of larger teams).
func resolveTeamMembership(ctx context.Context, client graphql.Client, cache *ResolverCache, teamRef, userRef string) (string, error) {
	teamID, err := resolveTeam(ctx, client, cache, teamRef)
	if err != nil {
		return "", err
	}
	userID, err := resolveUser(ctx, client, cache, userRef)
	if err != nil {
		return "", err
	}

	pageSize := 250
	var after *string
	for {
		resp, err := api.GetTeamMemberships(ctx, client, teamID, &pageSize, after)
		if err != nil {
			return "", fmt.Errorf("failed to load team memberships: %w", err)
		}
		if resp.Team == nil || resp.Team.Memberships == nil {
			return "", fmt.Errorf("team not found: %s", teamRef)
		}

		for _, m := range resp.Team.Memberships.Nodes {
			if m.User != nil && m.User.Id == userID {
				return m.Id, nil
			}
		}

		pageInfo := resp.Team.Memberships.PageInfo
		if pageInfo == nil || !pageInfo.HasNextPage || pageInfo.EndCursor == nil {
			break
		}
		after = pageInfo.EndCursor
	}

	return "", fmt.Errorf("user %q is not a member of team %q", userRef, teamRef)
}

// resolveCycle resolves a cycle reference to a cycle ID within the given team.
// ref may be a UUID (returned as-is), a cycle number (e.g. "12"), or a cycle
// name; numbers and names are matched within teamID. A reference matching more
// than one cycle returns an error listing the candidates; a reference matching
// none returns a "not found" error.
func resolveCycle(ctx context.Context, client graphql.Client, cache *ResolverCache, teamID, ref string) (string, error) {
	if isUUID(ref) {
		return ref, nil
	}

	cacheKey := teamID + ":" + strings.ToLower(ref)
	if id, ok := cache.cycles[cacheKey]; ok {
		return id, nil
	}

	filter := &api.CycleFilter{
		Team: &api.TeamFilter{Id: &api.IDComparator{Eq: &teamID}},
	}
	// A bare number selects by cycle number; anything else matches by name.
	if num, err := strconv.ParseFloat(ref, 64); err == nil {
		filter.Number = &api.NumberComparator{Eq: &num}
	} else {
		filter.Name = &api.StringComparator{EqIgnoreCase: &ref}
	}

	limit := 10
	resp, err := api.ListCycles(ctx, client, filter, &limit, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find cycle '%s': %w", ref, err)
	}

	nodes := resp.Cycles.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Cycle not found in team: %s", ref)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%.0f (%s)", n.CycleListFields.Number, n.CycleListFields.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", ref, strings.Join(candidates, ", "))
	}

	id := nodes[0].CycleListFields.Id
	cache.cycles[cacheKey] = id
	return id, nil
}

// resolveTemplate resolves a template name or UUID to a template ID, scoped to
// a template type ("issue" or "project"). The `templates` query returns the
// whole workspace list with no server-side filter, so matching is done here:
// case-insensitive on name, restricted to the given type. A name matching more
// than one template of that type returns an error listing the candidates; a
// name matching none returns a "not found" error.
func resolveTemplate(ctx context.Context, client graphql.Client, cache *ResolverCache, templateType, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := templateType + ":" + strings.ToLower(nameOrID)
	if id, ok := cache.templates[cacheKey]; ok {
		return id, nil
	}

	resp, err := api.ListTemplates(ctx, client)
	if err != nil {
		return "", fmt.Errorf("failed to list templates: %w", err)
	}

	var matches []struct{ id, name string }
	for _, node := range resp.Templates {
		f := node.TemplateFields
		if !strings.EqualFold(f.Type, templateType) {
			continue
		}
		if strings.EqualFold(f.Name, nameOrID) {
			matches = append(matches, struct{ id, name string }{f.Id, f.Name})
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("%s template not found: %s", templateType, nameOrID)
	}
	if len(matches) > 1 {
		candidates := make([]string, 0, len(matches))
		for _, m := range matches {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", m.name, m.id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	cache.templates[cacheKey] = matches[0].id
	return matches[0].id, nil
}

// resolveCustomer resolves a customer name or UUID to a customer ID. Name
// matching is case-insensitive. A name matching more than one customer returns
// an error listing the candidates (name and id); a name matching no customer
// returns a "not found" error.
func resolveCustomer(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	cacheKey := strings.ToLower(nameOrID)
	if id, ok := cache.customers[cacheKey]; ok {
		return id, nil
	}

	filter := &api.CustomerFilter{
		Name: &api.StringComparator{EqIgnoreCase: &nameOrID},
	}
	limit := 10
	resp, err := api.ListCustomers(ctx, client, filter, &limit, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to find customer '%s': %w", nameOrID, err)
	}

	nodes := resp.Customers.Nodes
	if len(nodes) == 0 {
		return "", fmt.Errorf("Customer not found: %s", nameOrID)
	}
	if len(nodes) > 1 {
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%s (%s)", n.CustomerFields.Name, n.CustomerFields.Id))
		}
		return "", fmt.Errorf("Multiple matches for '%s': %s", nameOrID, strings.Join(candidates, ", "))
	}

	id := nodes[0].CustomerFields.Id
	cache.customers[cacheKey] = id
	return id, nil
}

// resolveCustomerStatus resolves a customer status name or UUID to a
// CustomerStatus ID. The status catalog is workspace-wide and small, so it is
// fetched at most once per invocation and matched case-insensitively against
// both name and displayName. A name matching none returns an error listing the
// valid status names.
func resolveCustomerStatus(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	if !cache.customerStatusesLoaded {
		limit := 250
		resp, err := api.ListCustomerStatuses(ctx, client, &limit, nil, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list customer statuses: %w", err)
		}
		if resp.CustomerStatuses != nil {
			for _, n := range resp.CustomerStatuses.Nodes {
				f := n.CustomerStatusFields
				cache.customerStatuses[strings.ToLower(f.Name)] = f.Id
				cache.customerStatuses[strings.ToLower(f.DisplayName)] = f.Id
			}
		}
		cache.customerStatusesLoaded = true
	}

	if id, ok := cache.customerStatuses[strings.ToLower(nameOrID)]; ok {
		return id, nil
	}

	names := make([]string, 0, len(cache.customerStatuses))
	for name := range cache.customerStatuses {
		names = append(names, name)
	}
	return "", fmt.Errorf("Customer status '%s' not found. Valid values: %s", nameOrID, strings.Join(names, ", "))
}

// resolveCustomerTier resolves a customer tier name or UUID to a CustomerTier
// ID, mirroring resolveCustomerStatus: the workspace tier catalog is fetched at
// most once and matched case-insensitively against name and displayName.
func resolveCustomerTier(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	if !cache.customerTiersLoaded {
		limit := 250
		resp, err := api.ListCustomerTiers(ctx, client, &limit, nil, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list customer tiers: %w", err)
		}
		if resp.CustomerTiers != nil {
			for _, n := range resp.CustomerTiers.Nodes {
				f := n.CustomerTierFields
				cache.customerTiers[strings.ToLower(f.Name)] = f.Id
				cache.customerTiers[strings.ToLower(f.DisplayName)] = f.Id
			}
		}
		cache.customerTiersLoaded = true
	}

	if id, ok := cache.customerTiers[strings.ToLower(nameOrID)]; ok {
		return id, nil
	}

	names := make([]string, 0, len(cache.customerTiers))
	for name := range cache.customerTiers {
		names = append(names, name)
	}
	return "", fmt.Errorf("Customer tier '%s' not found. Valid values: %s", nameOrID, strings.Join(names, ", "))
}

// resolveTimeSchedule resolves a time schedule name or UUID to a TimeSchedule
// ID. The timeSchedules query has no server-side name filter, so the whole
// (small) catalog is fetched at most once per invocation and matched
// case-insensitively by name. A name matching none returns a "not found" error
// listing the available schedule names.
func resolveTimeSchedule(ctx context.Context, client graphql.Client, cache *ResolverCache, nameOrID string) (string, error) {
	if isUUID(nameOrID) {
		return nameOrID, nil
	}

	if !cache.timeSchedulesLoaded {
		limit := 250
		resp, err := api.ListTimeSchedules(ctx, client, &limit, nil, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list time schedules: %w", err)
		}
		if resp.TimeSchedules != nil {
			for _, n := range resp.TimeSchedules.Nodes {
				f := n.TimeScheduleFields
				cache.timeSchedules[strings.ToLower(f.Name)] = f.Id
			}
		}
		cache.timeSchedulesLoaded = true
	}

	if id, ok := cache.timeSchedules[strings.ToLower(nameOrID)]; ok {
		return id, nil
	}

	names := make([]string, 0, len(cache.timeSchedules))
	for name := range cache.timeSchedules {
		names = append(names, name)
	}
	return "", fmt.Errorf("Time schedule not found: %s. Available: %s", nameOrID, strings.Join(names, ", "))
}

package api

import (
	"context"
	"fmt"
)

// This file provides adapter functions that wrap genqlient-generated code
// while maintaining backward compatibility with the existing CLI command API.
// These adapters convert between the old map-based filter API and the new
// type-safe genqlient types.

// GetIssuesNew wraps the generated ListIssues function
func (c *Client) GetIssues(ctx context.Context, filter map[string]interface{}, first int, after string, orderBy string) (*Issues, error) {
	// Convert map filter to typed IssueFilter
	issueFilter, err := convertToIssueFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter: %w", err)
	}

	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	// Call generated function
	resp, err := ListIssues(ctx, c, issueFilter, firstPtr, afterPtr, orderByEnum)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issues type
	return convertToLegacyIssues(resp), nil
}

// GetIssueNew wraps the generated GetIssue function
func (c *Client) GetIssue(ctx context.Context, id string) (*Issue, error) {
	// Call generated function
	resp, err := GetIssue(ctx, c, id)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issue type
	return convertToLegacyIssue(resp), nil
}

// IssueSearchNew wraps the generated SearchIssues function
func (c *Client) IssueSearch(ctx context.Context, term string, filter map[string]interface{}, first int, after string, orderBy string, includeArchived bool) (*Issues, error) {
	// Convert map filter to typed IssueFilter
	issueFilter, err := convertToIssueFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter: %w", err)
	}

	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	includeArchivedPtr := &includeArchived

	// Call generated function
	resp, err := SearchIssues(ctx, c, term, issueFilter, firstPtr, afterPtr, orderByEnum, includeArchivedPtr)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issues type
	return convertToLegacyIssuesFromSearch(resp), nil
}

// CreateIssueNew wraps the generated CreateIssue function
func (c *Client) CreateIssue(ctx context.Context, input map[string]interface{}) (*Issue, error) {
	// Convert map input to typed IssueCreateInput
	createInput, err := convertToIssueCreateInput(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert create input: %w", err)
	}

	// Call generated function
	resp, err := CreateIssue(ctx, c, createInput)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issue type
	return convertToLegacyIssueFromCreate(resp), nil
}

// UpdateIssueNew wraps the generated UpdateIssue function
func (c *Client) UpdateIssue(ctx context.Context, id string, input map[string]interface{}) (*Issue, error) {
	// Convert map input to typed IssueUpdateInput
	updateInput, err := convertToIssueUpdateInput(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert update input: %w", err)
	}

	// Call generated function
	resp, err := UpdateIssue(ctx, c, id, updateInput)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issue type
	return convertToLegacyIssueFromUpdate(resp), nil
}

// Helper conversion functions

// convertToIssueFilter converts a map filter to a typed IssueFilter
func convertToIssueFilter(filter map[string]interface{}) (*IssueFilter, error) {
	if filter == nil {
		return nil, nil
	}

	// For now, we'll use a simple approach: marshal to JSON and unmarshal to typed filter
	// This maintains compatibility while providing type safety
	// In the future, we can make this more sophisticated
	issueFilter := &IssueFilter{}

	// Handle common filter fields
	if assignee, ok := filter["assignee"].(map[string]interface{}); ok {
		issueFilter.Assignee = convertToNullableUserFilter(assignee)
	}

	if team, ok := filter["team"].(map[string]interface{}); ok {
		issueFilter.Team = convertToTeamFilter(team)
	}

	if state, ok := filter["state"].(map[string]interface{}); ok {
		issueFilter.State = convertToWorkflowStateFilter(state)
	}

	if priority, ok := filter["priority"].(map[string]interface{}); ok {
		issueFilter.Priority = convertToNullableNumberComparator(priority)
	}

	if createdAt, ok := filter["createdAt"].(map[string]interface{}); ok {
		issueFilter.CreatedAt = convertToDateComparator(createdAt)
	}

	if updatedAt, ok := filter["updatedAt"].(map[string]interface{}); ok {
		issueFilter.UpdatedAt = convertToDateComparator(updatedAt)
	}

	return issueFilter, nil
}

// convertToNullableUserFilter converts a user filter map to typed filter
func convertToNullableUserFilter(filter map[string]interface{}) *NullableUserFilter {
	if filter == nil {
		return nil
	}

	userFilter := &NullableUserFilter{}

	if id, ok := filter["id"].(map[string]interface{}); ok {
		userFilter.Id = convertToIDComparator(id)
	}

	if email, ok := filter["email"].(map[string]interface{}); ok {
		userFilter.Email = convertToStringComparator(email)
	}

	if isMe, ok := filter["isMe"].(map[string]interface{}); ok {
		userFilter.IsMe = convertToBooleanComparator(isMe)
	}

	return userFilter
}

// convertToTeamFilter converts a team filter map to typed filter
func convertToTeamFilter(filter map[string]interface{}) *TeamFilter {
	if filter == nil {
		return nil
	}

	teamFilter := &TeamFilter{}

	if id, ok := filter["id"].(map[string]interface{}); ok {
		teamFilter.Id = convertToIDComparator(id)
	}

	if key, ok := filter["key"].(map[string]interface{}); ok {
		teamFilter.Key = convertToStringComparator(key)
	}

	return teamFilter
}

// convertToWorkflowStateFilter converts a workflow state filter map to typed filter
func convertToWorkflowStateFilter(filter map[string]interface{}) *WorkflowStateFilter {
	if filter == nil {
		return nil
	}

	stateFilter := &WorkflowStateFilter{}
	hasValue := false

	if id, ok := filter["id"].(map[string]interface{}); ok {
		stateFilter.Id = convertToIDComparator(id)
		if stateFilter.Id != nil {
			hasValue = true
		}
	}

	if name, ok := filter["name"].(map[string]interface{}); ok {
		stateFilter.Name = convertToStringComparator(name)
		if stateFilter.Name != nil {
			hasValue = true
		}
	}

	if typeVal, ok := filter["type"].(map[string]interface{}); ok {
		stateFilter.Type = convertToStringComparator(typeVal)
		if stateFilter.Type != nil {
			hasValue = true
		}
	}

	if !hasValue {
		return nil
	}

	return stateFilter
}

// convertToIDComparator converts an ID comparator map to typed comparator
func convertToIDComparator(comp map[string]interface{}) *IDComparator {
	if comp == nil {
		return nil
	}

	comparator := &IDComparator{}

	if eq, ok := comp["eq"].(string); ok {
		comparator.Eq = &eq
	}

	if neq, ok := comp["neq"].(string); ok {
		comparator.Neq = &neq
	}

	if in, ok := comp["in"].([]interface{}); ok {
		stringSlice := make([]string, len(in))
		for i, v := range in {
			if s, ok := v.(string); ok {
				stringSlice[i] = s
			}
		}
		comparator.In = stringSlice
	}

	return comparator
}

// convertToBooleanComparator converts a boolean comparator map to typed comparator
func convertToBooleanComparator(comp map[string]interface{}) *BooleanComparator {
	if comp == nil {
		return nil
	}

	comparator := &BooleanComparator{}

	if eq, ok := comp["eq"].(bool); ok {
		comparator.Eq = &eq
	}

	if neq, ok := comp["neq"].(bool); ok {
		comparator.Neq = &neq
	}

	return comparator
}

// convertToStringComparator converts a string comparator map to typed comparator
func convertToStringComparator(comp map[string]interface{}) *StringComparator {
	if comp == nil {
		return nil
	}

	comparator := &StringComparator{}
	hasValue := false

	if eq, ok := comp["eq"].(string); ok {
		comparator.Eq = &eq
		hasValue = true
	}

	if neq, ok := comp["neq"].(string); ok {
		comparator.Neq = &neq
		hasValue = true
	}

	if contains, ok := comp["contains"].(string); ok {
		comparator.Contains = &contains
		hasValue = true
	}

	if containsIgnoreCase, ok := comp["containsIgnoreCase"].(string); ok {
		comparator.ContainsIgnoreCase = &containsIgnoreCase
		hasValue = true
	}

	if nin, ok := comp["nin"].([]interface{}); ok {
		stringSlice := make([]string, len(nin))
		for i, v := range nin {
			if s, ok := v.(string); ok {
				stringSlice[i] = s
			}
		}
		comparator.Nin = stringSlice
		hasValue = true
	}

	if !hasValue {
		return nil
	}

	return comparator
}

// convertToNullableNumberComparator converts a number comparator map to typed comparator
func convertToNullableNumberComparator(comp map[string]interface{}) *NullableNumberComparator {
	if comp == nil {
		return nil
	}

	comparator := &NullableNumberComparator{}

	if eq, ok := comp["eq"].(float64); ok {
		comparator.Eq = &eq
	}

	if neq, ok := comp["neq"].(float64); ok {
		comparator.Neq = &neq
	}

	if gt, ok := comp["gt"].(float64); ok {
		comparator.Gt = &gt
	}

	if gte, ok := comp["gte"].(float64); ok {
		comparator.Gte = &gte
	}

	if lt, ok := comp["lt"].(float64); ok {
		comparator.Lt = &lt
	}

	if lte, ok := comp["lte"].(float64); ok {
		comparator.Lte = &lte
	}

	return comparator
}

// convertToDateComparator converts a date comparator map to typed comparator
func convertToDateComparator(comp map[string]interface{}) *DateComparator {
	if comp == nil {
		return nil
	}

	comparator := &DateComparator{}
	hasValue := false

	// TimelessDate is configured as string in genqlient.yaml
	if eq, ok := comp["eq"].(string); ok {
		comparator.Eq = &eq
		hasValue = true
	}

	if neq, ok := comp["neq"].(string); ok {
		comparator.Neq = &neq
		hasValue = true
	}

	if gt, ok := comp["gt"].(string); ok {
		comparator.Gt = &gt
		hasValue = true
	}

	if gte, ok := comp["gte"].(string); ok {
		comparator.Gte = &gte
		hasValue = true
	}

	if lt, ok := comp["lt"].(string); ok {
		comparator.Lt = &lt
		hasValue = true
	}

	if lte, ok := comp["lte"].(string); ok {
		comparator.Lte = &lte
		hasValue = true
	}

	if !hasValue {
		return nil
	}

	return comparator
}

// convertOrderBy converts an orderBy string to PaginationOrderBy enum
func convertOrderBy(orderBy string) *PaginationOrderBy {
	if orderBy == "" {
		return nil
	}

	switch orderBy {
	case "createdAt":
		val := PaginationOrderByCreatedat
		return &val
	case "updatedAt":
		val := PaginationOrderByUpdatedat
		return &val
	default:
		return nil
	}
}

// convertToLegacyIssues converts a ListIssuesResponse to legacy Issues type
func convertToLegacyIssues(resp *ListIssuesResponse) *Issues {
	if resp == nil || resp.Issues == nil {
		return &Issues{
			Nodes:    []Issue{},
			PageInfo: PageInfo{},
		}
	}

	issues := make([]Issue, len(resp.Issues.Nodes))
	for i, node := range resp.Issues.Nodes {
		issues[i] = convertIssueListFieldsToLegacy(&node.IssueListFields)
	}

	endCursor := ""
	if resp.Issues.PageInfo.EndCursor != nil {
		endCursor = *resp.Issues.PageInfo.EndCursor
	}

	return &Issues{
		Nodes: issues,
		PageInfo: PageInfo{
			HasNextPage: resp.Issues.PageInfo.HasNextPage,
			EndCursor:   endCursor,
		},
	}
}

// convertToLegacyIssuesFromSearch converts a SearchIssuesResponse to legacy Issues type
func convertToLegacyIssuesFromSearch(resp *SearchIssuesResponse) *Issues {
	if resp == nil || resp.SearchIssues == nil {
		return &Issues{
			Nodes:    []Issue{},
			PageInfo: PageInfo{},
		}
	}

	issues := make([]Issue, len(resp.SearchIssues.Nodes))
	for i, node := range resp.SearchIssues.Nodes {
		issues[i] = convertSearchResultToLegacy(node)
	}

	endCursor := ""
	if resp.SearchIssues.PageInfo.EndCursor != nil {
		endCursor = *resp.SearchIssues.PageInfo.EndCursor
	}

	return &Issues{
		Nodes: issues,
		PageInfo: PageInfo{
			HasNextPage: resp.SearchIssues.PageInfo.HasNextPage,
			EndCursor:   endCursor,
		},
	}
}

// convertSearchResultToLegacy converts SearchIssuesSearchIssuesIssueSearchPayloadNodesIssueSearchResult to legacy Issue
func convertSearchResultToLegacy(result *SearchIssuesSearchIssuesIssueSearchPayloadNodesIssueSearchResult) Issue {
	issue := Issue{
		ID:          result.Id,
		Identifier:  result.Identifier,
		Title:       result.Title,
		Description: ptrToString(result.Description),
		Priority:    int(result.Priority),
		Estimate:    result.Estimate,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
		DueDate:     result.DueDate,
		URL:         result.Url,
	}

	if result.State != nil {
		issue.State = &State{
			ID:    result.State.Id,
			Name:  result.State.Name,
			Type:  result.State.Type,
			Color: result.State.Color,
		}
	}

	if result.Assignee != nil {
		issue.Assignee = &User{
			ID:    result.Assignee.Id,
			Name:  result.Assignee.Name,
			Email: result.Assignee.Email,
		}
	}

	if result.Team != nil {
		issue.Team = &Team{
			ID:   result.Team.Id,
			Key:  result.Team.Key,
			Name: result.Team.Name,
		}
	}

	if result.Labels != nil && len(result.Labels.Nodes) > 0 {
		labels := make([]Label, len(result.Labels.Nodes))
		for i, label := range result.Labels.Nodes {
			labels[i] = Label{
				ID:    label.Id,
				Name:  label.Name,
				Color: label.Color,
			}
		}
		issue.Labels = &Labels{Nodes: labels}
	}

	return issue
}

// convertIssueListFieldsToLegacy converts IssueListFields to legacy Issue
func convertIssueListFieldsToLegacy(fields *IssueListFields) Issue {
	issue := Issue{
		ID:          fields.Id,
		Identifier:  fields.Identifier,
		Title:       fields.Title,
		Description: ptrToString(fields.Description),
		Priority:    int(fields.Priority),
		Estimate:    fields.Estimate,
		CreatedAt:   fields.CreatedAt,
		UpdatedAt:   fields.UpdatedAt,
		DueDate:     fields.DueDate,
		URL:         fields.Url,
	}

	if fields.State != nil {
		issue.State = &State{
			ID:    fields.State.Id,
			Name:  fields.State.Name,
			Type:  fields.State.Type,
			Color: fields.State.Color,
		}
	}

	if fields.Assignee != nil {
		issue.Assignee = &User{
			ID:    fields.Assignee.Id,
			Name:  fields.Assignee.Name,
			Email: fields.Assignee.Email,
		}
	}

	if fields.Team != nil {
		issue.Team = &Team{
			ID:   fields.Team.Id,
			Key:  fields.Team.Key,
			Name: fields.Team.Name,
		}
	}

	if fields.Labels != nil && len(fields.Labels.Nodes) > 0 {
		labels := make([]Label, len(fields.Labels.Nodes))
		for i, label := range fields.Labels.Nodes {
			labels[i] = Label{
				ID:    label.Id,
				Name:  label.Name,
				Color: label.Color,
			}
		}
		issue.Labels = &Labels{Nodes: labels}
	}

	return issue
}

// convertToLegacyIssue converts a GetIssueResponse to legacy Issue type
func convertToLegacyIssue(resp *GetIssueResponse) *Issue {
	if resp == nil || resp.Issue == nil {
		return nil
	}

	fields := &resp.Issue.IssueDetailFields
	issue := Issue{
		ID:                  fields.Id,
		Identifier:          fields.Identifier,
		Number:              int(fields.Number),
		Title:               fields.Title,
		Description:         ptrToString(fields.Description),
		Priority:            int(fields.Priority),
		PriorityLabel:       fields.PriorityLabel,
		Estimate:            fields.Estimate,
		BoardOrder:          fields.BoardOrder,
		SubIssueSortOrder:   ptrFloat64ToFloat64(fields.SubIssueSortOrder),
		CreatedAt:           fields.CreatedAt,
		UpdatedAt:           fields.UpdatedAt,
		DueDate:             fields.DueDate,
		URL:                 fields.Url,
		BranchName:          fields.BranchName,
		SnoozedUntilAt:      fields.SnoozedUntilAt,
		CompletedAt:         fields.CompletedAt,
		CanceledAt:          fields.CanceledAt,
		ArchivedAt:          fields.ArchivedAt,
		TriagedAt:           fields.TriagedAt,
		CustomerTicketCount: int(fields.CustomerTicketCount),
		PreviousIdentifiers: fields.PreviousIdentifiers,
	}

	// Convert nested structures
	if fields.State != nil {
		issue.State = &State{
			ID:          fields.State.Id,
			Name:        fields.State.Name,
			Type:        fields.State.Type,
			Color:       fields.State.Color,
			Description: fields.State.Description,
			Position:    fields.State.Position,
		}
	}

	if fields.Assignee != nil {
		issue.Assignee = &User{
			ID:          fields.Assignee.Id,
			Name:        fields.Assignee.Name,
			Email:       fields.Assignee.Email,
			AvatarURL:   ptrToString(fields.Assignee.AvatarUrl),
			DisplayName: fields.Assignee.DisplayName,
			Active:      fields.Assignee.Active,
			Admin:       fields.Assignee.Admin,
			CreatedAt:   &fields.Assignee.CreatedAt,
		}
	}

	if fields.Team != nil {
		issue.Team = &Team{
			ID:                 fields.Team.Id,
			Key:                fields.Team.Key,
			Name:               fields.Team.Name,
			Description:        ptrToString(fields.Team.Description),
			Icon:               fields.Team.Icon,
			Color:              ptrToString(fields.Team.Color),
			CyclesEnabled:      fields.Team.CyclesEnabled,
			CycleStartDay:      int(fields.Team.CycleStartDay),
			CycleDuration:      int(fields.Team.CycleDuration),
			UpcomingCycleCount: int(fields.Team.UpcomingCycleCount),
		}
	}

	// Additional conversions for detailed fields can be added here as needed
	// For now, we'll focus on the most commonly used fields

	return &issue
}

// convertToLegacyIssueFromCreate converts CreateIssueResponse to legacy Issue
func convertToLegacyIssueFromCreate(resp *CreateIssueResponse) *Issue {
	if resp == nil || resp.IssueCreate == nil || resp.IssueCreate.Issue == nil {
		return nil
	}

	fields := &resp.IssueCreate.Issue.IssueListFields
	issue := convertIssueListFieldsToLegacy(fields)
	return &issue
}

// convertToLegacyIssueFromUpdate converts UpdateIssueResponse to legacy Issue
func convertToLegacyIssueFromUpdate(resp *UpdateIssueResponse) *Issue {
	if resp == nil || resp.IssueUpdate == nil || resp.IssueUpdate.Issue == nil {
		return nil
	}

	fields := &resp.IssueUpdate.Issue.IssueListFields
	issue := convertIssueListFieldsToLegacy(fields)
	return &issue
}

// convertToIssueCreateInput converts a map to IssueCreateInput
func convertToIssueCreateInput(input map[string]interface{}) (*IssueCreateInput, error) {
	if input == nil {
		return nil, nil
	}

	createInput := &IssueCreateInput{}

	if teamId, ok := input["teamId"].(string); ok {
		createInput.TeamId = teamId
	}

	if title, ok := input["title"].(string); ok {
		createInput.Title = &title
	}

	if description, ok := input["description"].(string); ok {
		createInput.Description = &description
	}

	if assigneeId, ok := input["assigneeId"].(string); ok {
		createInput.AssigneeId = &assigneeId
	}

	if stateId, ok := input["stateId"].(string); ok {
		createInput.StateId = &stateId
	}

	if priority, ok := input["priority"].(int); ok {
		createInput.Priority = &priority
	}

	if estimate, ok := input["estimate"].(int); ok {
		createInput.Estimate = &estimate
	}

	if dueDate, ok := input["dueDate"].(string); ok {
		createInput.DueDate = &dueDate
	}

	if parentId, ok := input["parentId"].(string); ok {
		createInput.ParentId = &parentId
	}

	if projectId, ok := input["projectId"].(string); ok {
		createInput.ProjectId = &projectId
	}

	if cycleId, ok := input["cycleId"].(string); ok {
		createInput.CycleId = &cycleId
	}

	if labelIds, ok := input["labelIds"].([]interface{}); ok {
		strSlice := make([]string, len(labelIds))
		for i, v := range labelIds {
			if s, ok := v.(string); ok {
				strSlice[i] = s
			}
		}
		createInput.LabelIds = strSlice
	}

	return createInput, nil
}

// convertToIssueUpdateInput converts a map to IssueUpdateInput
func convertToIssueUpdateInput(input map[string]interface{}) (*IssueUpdateInput, error) {
	if input == nil {
		return nil, nil
	}

	updateInput := &IssueUpdateInput{}

	if title, ok := input["title"].(string); ok {
		updateInput.Title = &title
	}

	if description, ok := input["description"].(string); ok {
		updateInput.Description = &description
	}

	if assigneeId, ok := input["assigneeId"].(string); ok {
		updateInput.AssigneeId = &assigneeId
	}

	if stateId, ok := input["stateId"].(string); ok {
		updateInput.StateId = &stateId
	}

	if priority, ok := input["priority"].(int); ok {
		updateInput.Priority = &priority
	}

	if estimate, ok := input["estimate"].(int); ok {
		updateInput.Estimate = &estimate
	}

	if dueDate, ok := input["dueDate"].(string); ok {
		updateInput.DueDate = &dueDate
	}

	if parentId, ok := input["parentId"].(string); ok {
		updateInput.ParentId = &parentId
	}

	if projectId, ok := input["projectId"].(string); ok {
		updateInput.ProjectId = &projectId
	}

	if cycleId, ok := input["cycleId"].(string); ok {
		updateInput.CycleId = &cycleId
	}

	if labelIds, ok := input["labelIds"].([]interface{}); ok {
		strSlice := make([]string, len(labelIds))
		for i, v := range labelIds {
			if s, ok := v.(string); ok {
				strSlice[i] = s
			}
		}
		updateInput.LabelIds = strSlice
	}

	return updateInput, nil
}

// Helper functions to convert pointer to value
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptrFloat64ToFloat64(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

func convertHealthToString(h *ProjectUpdateHealthType) string {
	if h == nil {
		return ""
	}
	return string(*h)
}

// ========== Project Adapters ==========

// GetProjectsNew wraps the generated ListProjects function
func (c *Client) GetProjects(ctx context.Context, filter map[string]interface{}, first int, after string, orderBy string) (*Projects, error) {
	// Convert map filter to typed ProjectFilter
	projectFilter, err := convertToProjectFilter(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to convert filter: %w", err)
	}

	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	// Call generated function
	resp, err := ListProjects(ctx, c, projectFilter, firstPtr, afterPtr, orderByEnum)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Projects type
	return convertToLegacyProjects(resp), nil
}

// GetProjectNew wraps the generated GetProject function
func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	// Call generated function
	resp, err := GetProject(ctx, c, id)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Project type
	return convertToLegacyProject(resp), nil
}

// convertToProjectFilter converts a map filter to a typed ProjectFilter
func convertToProjectFilter(filter map[string]interface{}) (*ProjectFilter, error) {
	if filter == nil {
		return nil, nil
	}

	projectFilter := &ProjectFilter{}

	// Handle common filter fields
	if team, ok := filter["team"].(map[string]interface{}); ok {
		teamFilter := convertToTeamFilter(team)
		if teamFilter != nil {
			projectFilter.AccessibleTeams = &TeamCollectionFilter{
				Some: teamFilter,
			}
		}
	}

	if state, ok := filter["state"].(map[string]interface{}); ok {
		projectFilter.State = convertToProjectStateFilter(state)
	}

	if createdAt, ok := filter["createdAt"].(map[string]interface{}); ok {
		projectFilter.CreatedAt = convertToDateComparator(createdAt)
	}

	if updatedAt, ok := filter["updatedAt"].(map[string]interface{}); ok {
		projectFilter.UpdatedAt = convertToDateComparator(updatedAt)
	}

	return projectFilter, nil
}

// convertToProjectStateFilter converts a project state filter map to typed filter
func convertToProjectStateFilter(filter map[string]interface{}) *StringComparator {
	if filter == nil {
		return nil
	}

	// Project state filter is just a StringComparator in Linear's API
	return convertToStringComparator(filter)
}

// convertToLegacyProjects converts a ListProjectsResponse to legacy Projects type
func convertToLegacyProjects(resp *ListProjectsResponse) *Projects {
	if resp == nil || resp.Projects == nil {
		return &Projects{
			Nodes:    []Project{},
			PageInfo: PageInfo{},
		}
	}

	projects := make([]Project, len(resp.Projects.Nodes))
	for i, node := range resp.Projects.Nodes {
		projects[i] = convertProjectListFieldsToLegacy(&node.ProjectListFields)
	}

	endCursor := ""
	if resp.Projects.PageInfo.EndCursor != nil {
		endCursor = *resp.Projects.PageInfo.EndCursor
	}

	return &Projects{
		Nodes: projects,
		PageInfo: PageInfo{
			HasNextPage: resp.Projects.PageInfo.HasNextPage,
			EndCursor:   endCursor,
		},
	}
}

// convertProjectListFieldsToLegacy converts ProjectListFields to legacy Project
func convertProjectListFieldsToLegacy(fields *ProjectListFields) Project {
	project := Project{
		ID:          fields.Id,
		Name:        fields.Name,
		Description: fields.Description,
		State:       fields.State,
		Progress:    fields.Progress,
		StartDate:   fields.StartDate,
		TargetDate:  fields.TargetDate,
		URL:         fields.Url,
		CreatedAt:   fields.CreatedAt,
		UpdatedAt:   fields.UpdatedAt,
	}

	if fields.Lead != nil {
		project.Lead = &User{
			ID:    fields.Lead.Id,
			Name:  fields.Lead.Name,
			Email: fields.Lead.Email,
		}
	}

	if fields.Teams != nil && len(fields.Teams.Nodes) > 0 {
		teams := make([]Team, len(fields.Teams.Nodes))
		for i, team := range fields.Teams.Nodes {
			teams[i] = Team{
				ID:   team.Id,
				Key:  team.Key,
				Name: team.Name,
			}
		}
		project.Teams = &Teams{Nodes: teams}
	}

	return project
}

// convertToLegacyProject converts a GetProjectResponse to legacy Project type
func convertToLegacyProject(resp *GetProjectResponse) *Project {
	if resp == nil || resp.Project == nil {
		return nil
	}

	fields := &resp.Project.ProjectDetailFields
	project := Project{
		ID:                  fields.Id,
		Name:                fields.Name,
		Description:         fields.Description,
		State:               fields.State,
		Progress:            fields.Progress,
		StartDate:           fields.StartDate,
		TargetDate:          fields.TargetDate,
		URL:                 fields.Url,
		Icon:                fields.Icon,
		Color:               fields.Color,
		CreatedAt:           fields.CreatedAt,
		UpdatedAt:           fields.UpdatedAt,
		CompletedAt:         fields.CompletedAt,
		CanceledAt:          fields.CanceledAt,
		ArchivedAt:          fields.ArchivedAt,
		SlugId:              fields.SlugId,
		Content:             ptrToString(fields.Content),
		Health:              convertHealthToString(fields.Health),
		Scope:               int(fields.Scope),
		SlackNewIssue:       fields.SlackNewIssue,
		SlackIssueComments:  fields.SlackIssueComments,
		SlackIssueStatuses:  fields.SlackIssueStatuses,
	}

	if fields.Lead != nil {
		project.Lead = &User{
			ID:          fields.Lead.Id,
			Name:        fields.Lead.Name,
			Email:       fields.Lead.Email,
			AvatarURL:   ptrToString(fields.Lead.AvatarUrl),
			DisplayName: fields.Lead.DisplayName,
			Active:      fields.Lead.Active,
		}
	}

	if fields.Creator != nil {
		project.Creator = &User{
			ID:        fields.Creator.Id,
			Name:      fields.Creator.Name,
			Email:     fields.Creator.Email,
			AvatarURL: ptrToString(fields.Creator.AvatarUrl),
			Active:    fields.Creator.Active,
		}
	}

	if fields.ConvertedFromIssue != nil {
		project.ConvertedFromIssue = &Issue{
			ID:         fields.ConvertedFromIssue.Id,
			Identifier: fields.ConvertedFromIssue.Identifier,
			Title:      fields.ConvertedFromIssue.Title,
		}
	}

	if fields.LastAppliedTemplate != nil {
		project.LastAppliedTemplate = &Template{
			ID:          fields.LastAppliedTemplate.Id,
			Name:        fields.LastAppliedTemplate.Name,
			Description: ptrToString(fields.LastAppliedTemplate.Description),
		}
	}

	if fields.Teams != nil && len(fields.Teams.Nodes) > 0 {
		teams := make([]Team, len(fields.Teams.Nodes))
		for i, team := range fields.Teams.Nodes {
			teams[i] = Team{
				ID:            team.Id,
				Key:           team.Key,
				Name:          team.Name,
				Description:   ptrToString(team.Description),
				Icon:          team.Icon,
				Color:         ptrToString(team.Color),
				CyclesEnabled: team.CyclesEnabled,
			}
		}
		project.Teams = &Teams{Nodes: teams}
	}

	if fields.Members != nil && len(fields.Members.Nodes) > 0 {
		users := make([]User, len(fields.Members.Nodes))
		for i, member := range fields.Members.Nodes {
			users[i] = User{
				ID:          member.Id,
				Name:        member.Name,
				Email:       member.Email,
				AvatarURL:   ptrToString(member.AvatarUrl),
				DisplayName: member.DisplayName,
				Active:      member.Active,
				Admin:       member.Admin,
			}
		}
		project.Members = &Users{Nodes: users}
	}

	if fields.Issues != nil && len(fields.Issues.Nodes) > 0 {
		issues := make([]Issue, len(fields.Issues.Nodes))
		for i, issue := range fields.Issues.Nodes {
			issues[i] = Issue{
				ID:          issue.Id,
				Identifier:  issue.Identifier,
				Number:      int(issue.Number),
				Title:       issue.Title,
				Description: ptrToString(issue.Description),
				Priority:    int(issue.Priority),
				Estimate:    issue.Estimate,
				CreatedAt:   issue.CreatedAt,
				UpdatedAt:   issue.UpdatedAt,
				CompletedAt: issue.CompletedAt,
			}

			if issue.State != nil {
				issues[i].State = &State{
					Name:  issue.State.Name,
					Type:  issue.State.Type,
					Color: issue.State.Color,
				}
			}

			if issue.Assignee != nil {
				issues[i].Assignee = &User{
					Name:  issue.Assignee.Name,
					Email: issue.Assignee.Email,
				}
			}

			if issue.Labels != nil && len(issue.Labels.Nodes) > 0 {
				labels := make([]Label, len(issue.Labels.Nodes))
				for j, label := range issue.Labels.Nodes {
					labels[j] = Label{
						Name:  label.Name,
						Color: label.Color,
					}
				}
				issues[i].Labels = &Labels{Nodes: labels}
			}
		}
		project.Issues = &Issues{Nodes: issues}
	}

	if fields.ProjectUpdates != nil && len(fields.ProjectUpdates.Nodes) > 0 {
		updates := make([]ProjectUpdate, len(fields.ProjectUpdates.Nodes))
		for i, update := range fields.ProjectUpdates.Nodes {
			updates[i] = ProjectUpdate{
				ID:        update.Id,
				Body:      update.Body,
				Health:    string(update.Health),
				CreatedAt: update.CreatedAt,
				UpdatedAt: update.UpdatedAt,
				EditedAt:  update.EditedAt,
			}

			if update.User != nil {
				updates[i].User = &User{
					Name:      update.User.Name,
					Email:     update.User.Email,
					AvatarURL: ptrToString(update.User.AvatarUrl),
				}
			}
		}
		project.ProjectUpdates = &ProjectUpdates{Nodes: updates}
	}

	if fields.Documents != nil && len(fields.Documents.Nodes) > 0 {
		docs := make([]Document, len(fields.Documents.Nodes))
		for i, doc := range fields.Documents.Nodes {
			docs[i] = Document{
				ID:        doc.Id,
				Title:     doc.Title,
				Content:   ptrToString(doc.Content),
				Icon:      doc.Icon,
				Color:     ptrToString(doc.Color),
				CreatedAt: doc.CreatedAt,
				UpdatedAt: doc.UpdatedAt,
			}

			if doc.Creator != nil {
				docs[i].Creator = &User{
					Name:  doc.Creator.Name,
					Email: doc.Creator.Email,
				}
			}

			if doc.UpdatedBy != nil {
				docs[i].UpdatedBy = &User{
					Name:  doc.UpdatedBy.Name,
					Email: doc.UpdatedBy.Email,
				}
			}
		}
		project.Documents = &Documents{Nodes: docs}
	}

	return &project
}

// ========== Team Adapters ==========

// GetTeamsNew wraps the generated ListTeams function
func (c *Client) GetTeams(ctx context.Context, first int, after string, orderBy string) (*Teams, error) {
	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	// Call generated function
	resp, err := ListTeams(ctx, c, firstPtr, afterPtr, orderByEnum)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Teams type
	return convertToLegacyTeams(resp), nil
}

// GetTeamNew wraps the generated GetTeam function
func (c *Client) GetTeam(ctx context.Context, key string) (*Team, error) {
	// Call generated function
	resp, err := GetTeam(ctx, c, key)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Team type
	return convertToLegacyTeam(resp), nil
}

// convertToLegacyTeams converts a ListTeamsResponse to legacy Teams type
func convertToLegacyTeams(resp *ListTeamsResponse) *Teams {
	if resp == nil || resp.Teams == nil {
		return &Teams{
			Nodes:    []Team{},
			PageInfo: PageInfo{},
		}
	}

	teams := make([]Team, len(resp.Teams.Nodes))
	for i, node := range resp.Teams.Nodes {
		teams[i] = convertTeamListFieldsToLegacy(&node.TeamListFields)
	}

	endCursor := ""
	if resp.Teams.PageInfo.EndCursor != nil {
		endCursor = *resp.Teams.PageInfo.EndCursor
	}

	return &Teams{
		Nodes: teams,
		PageInfo: PageInfo{
			HasNextPage: resp.Teams.PageInfo.HasNextPage,
			EndCursor:   endCursor,
		},
	}
}

// convertTeamListFieldsToLegacy converts TeamListFields to legacy Team
func convertTeamListFieldsToLegacy(fields *TeamListFields) Team {
	description := ""
	if fields.Description != nil {
		description = *fields.Description
	}

	return Team{
		ID:          fields.Id,
		Key:         fields.Key,
		Name:        fields.Name,
		Description: description,
		Private:     fields.Private,
		IssueCount:  fields.IssueCount,
	}
}

// convertToLegacyTeam converts a GetTeamResponse to legacy Team type
func convertToLegacyTeam(resp *GetTeamResponse) *Team {
	if resp == nil || resp.Team == nil {
		return &Team{}
	}

	fields := resp.Team

	description := ""
	if fields.Description != nil {
		description = *fields.Description
	}

	var icon *string
	if fields.Icon != nil {
		icon = fields.Icon
	}

	color := ""
	if fields.Color != nil {
		color = *fields.Color
	}

	return &Team{
		ID:                 fields.Id,
		Key:                fields.Key,
		Name:               fields.Name,
		Description:        description,
		Icon:               icon,
		Color:              color,
		Private:            fields.Private,
		IssueCount:         fields.IssueCount,
		CyclesEnabled:      fields.CyclesEnabled,
		CycleStartDay:      int(fields.CycleStartDay),
		CycleDuration:      int(fields.CycleDuration),
		UpcomingCycleCount: int(fields.UpcomingCycleCount),
	}
}

// ========== User Adapters ==========

// GetUsersNew wraps the generated ListUsers function
func (c *Client) GetUsers(ctx context.Context, first int, after string, orderBy string) (*Users, error) {
	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	// Call generated function
	resp, err := ListUsers(ctx, c, firstPtr, afterPtr, orderByEnum)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Users type
	return convertToLegacyUsers(resp), nil
}

// GetUserNew wraps the generated GetUserByEmail function
func (c *Client) GetUser(ctx context.Context, email string) (*User, error) {
	// Create filter for email lookup
	filter := &UserFilter{
		Email: &StringComparator{
			Eq: &email,
		},
	}

	// Call generated function
	resp, err := GetUserByEmail(ctx, c, filter)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy User type
	// Since we're filtering by email with eq, we should get exactly one result
	if len(resp.Users.Nodes) == 0 {
		return nil, fmt.Errorf("user not found with email: %s", email)
	}

	return convertToLegacyUser(&resp.Users.Nodes[0].UserDetailFields), nil
}

// GetViewerNew wraps the generated GetViewer function
func (c *Client) GetViewer(ctx context.Context) (*User, error) {
	// Call generated function
	resp, err := GetViewer(ctx, c)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy User type
	return convertToLegacyUser(&resp.Viewer.UserDetailFields), nil
}

// convertToLegacyUsers converts a ListUsersResponse to legacy Users type
func convertToLegacyUsers(resp *ListUsersResponse) *Users {
	if resp == nil || resp.Users == nil {
		return &Users{
			Nodes:    []User{},
			PageInfo: PageInfo{},
		}
	}

	users := make([]User, len(resp.Users.Nodes))
	for i, node := range resp.Users.Nodes {
		users[i] = convertUserListFieldsToLegacy(&node.UserListFields)
	}

	endCursor := ""
	if resp.Users.PageInfo.EndCursor != nil {
		endCursor = *resp.Users.PageInfo.EndCursor
	}

	return &Users{
		Nodes: users,
		PageInfo: PageInfo{
			HasNextPage: resp.Users.PageInfo.HasNextPage,
			EndCursor:   endCursor,
		},
	}
}

// convertUserListFieldsToLegacy converts UserListFields to legacy User
func convertUserListFieldsToLegacy(fields *UserListFields) User {
	return User{
		ID:        fields.Id,
		Name:      fields.Name,
		Email:     fields.Email,
		AvatarURL: ptrToString(fields.AvatarUrl),
		IsMe:      fields.IsMe,
		Active:    fields.Active,
		Admin:     fields.Admin,
	}
}

// convertToLegacyUser converts UserDetailFields to legacy User type
func convertToLegacyUser(fields *UserDetailFields) *User {
	if fields == nil {
		return nil
	}

	return &User{
		ID:          fields.Id,
		Name:        fields.Name,
		Email:       fields.Email,
		AvatarURL:   ptrToString(fields.AvatarUrl),
		DisplayName: fields.DisplayName,
		IsMe:        fields.IsMe,
		Active:      fields.Active,
		Admin:       fields.Admin,
		CreatedAt:   &fields.CreatedAt,
	}
}

// ==============================================================================
// Comment Adapters
// ==============================================================================

// GetIssueCommentsNew wraps the generated ListComments function
func (c *Client) GetIssueComments(ctx context.Context, issueID string, first int, after string, orderBy string) (*Comments, error) {
	// Convert orderBy string to PaginationOrderBy
	orderByEnum := convertOrderBy(orderBy)

	// Convert parameters to pointers for genqlient
	var firstPtr *int
	if first > 0 {
		firstPtr = &first
	}

	var afterPtr *string
	if after != "" {
		afterPtr = &after
	}

	// Call generated function
	resp, err := ListComments(ctx, c, issueID, firstPtr, afterPtr, orderByEnum)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Comments type
	return convertToLegacyComments(resp), nil
}

// CreateCommentNew wraps the generated CreateComment function
func (c *Client) CreateComment(ctx context.Context, issueID string, body string) (*Comment, error) {
	// Create input
	input := &CommentCreateInput{
		IssueId: &issueID,
		Body:    &body,
	}

	// Call generated function
	resp, err := CreateComment(ctx, c, input)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Comment type
	return convertToLegacyCommentFromCreate(resp), nil
}

// UpdateCommentNew wraps the generated UpdateComment function
func (c *Client) UpdateComment(ctx context.Context, id string, body string) (*Comment, error) {
	// Create input
	input := &CommentUpdateInput{
		Body: &body,
	}

	// Call generated function
	resp, err := UpdateComment(ctx, c, id, input)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Comment type
	return convertToLegacyCommentFromUpdate(resp), nil
}

// Helper conversion functions for Comments

// convertToLegacyComments converts ListCommentsResponse to legacy Comments type
func convertToLegacyComments(resp *ListCommentsResponse) *Comments {
	if resp == nil || resp.Issue.Comments.Nodes == nil {
		return &Comments{
			Nodes:    []Comment{},
			PageInfo: PageInfo{},
		}
	}

	comments := make([]Comment, len(resp.Issue.Comments.Nodes))
	for i, node := range resp.Issue.Comments.Nodes {
		comments[i] = Comment{
			ID:        node.Id,
			Body:      node.Body,
			CreatedAt: node.CreatedAt,
			UpdatedAt: node.UpdatedAt,
			User: &User{
				ID:    node.User.Id,
				Name:  node.User.Name,
				Email: node.User.Email,
			},
		}
	}

	return &Comments{
		Nodes: comments,
		PageInfo: PageInfo{
			HasNextPage: resp.Issue.Comments.PageInfo.HasNextPage,
			EndCursor:   ptrToString(resp.Issue.Comments.PageInfo.EndCursor),
		},
	}
}

// convertToLegacyCommentFromCreate converts CreateCommentResponse to legacy Comment type
func convertToLegacyCommentFromCreate(resp *CreateCommentResponse) *Comment {
	if resp == nil || resp.CommentCreate.Comment == nil {
		return nil
	}

	fields := resp.CommentCreate.Comment
	return &Comment{
		ID:        fields.Id,
		Body:      fields.Body,
		CreatedAt: fields.CreatedAt,
		UpdatedAt: fields.UpdatedAt,
		EditedAt:  fields.EditedAt,
		User: &User{
			ID:        fields.User.Id,
			Name:      fields.User.Name,
			Email:     fields.User.Email,
			AvatarURL: ptrToString(fields.User.AvatarUrl),
		},
	}
}

// convertToLegacyCommentFromUpdate converts UpdateCommentResponse to legacy Comment type
func convertToLegacyCommentFromUpdate(resp *UpdateCommentResponse) *Comment {
	if resp == nil || resp.CommentUpdate.Comment == nil {
		return nil
	}

	fields := resp.CommentUpdate.Comment
	return &Comment{
		ID:        fields.Id,
		Body:      fields.Body,
		CreatedAt: fields.CreatedAt,
		UpdatedAt: fields.UpdatedAt,
		EditedAt:  fields.EditedAt,
		User: &User{
			ID:        fields.User.Id,
			Name:      fields.User.Name,
			Email:     fields.User.Email,
			AvatarURL: ptrToString(fields.User.AvatarUrl),
		},
	}
}

// GetTeamMembers wraps the generated GetTeamMembers function
func (c *Client) GetTeamMembers(ctx context.Context, teamKey string) (*Users, error) {
	resp, err := GetTeamMembers(ctx, c, teamKey)
	if err != nil {
		return nil, err
	}
	return convertTeamMembersToLegacyUsers(resp), nil
}

// convertTeamMembersToLegacyUsers converts GetTeamMembers response to legacy Users type
func convertTeamMembersToLegacyUsers(resp *GetTeamMembersResponse) *Users {
	users := &Users{
		Nodes: make([]User, len(resp.Team.Members.Nodes)),
	}

	for i, member := range resp.Team.Members.Nodes {
		users.Nodes[i] = User{
			ID:        member.Id,
			Name:      member.Name,
			Email:     member.Email,
			AvatarURL: ptrToString(member.AvatarUrl),
			IsMe:      member.IsMe,
			Active:    member.Active,
			Admin:     member.Admin,
		}
	}

	if resp.Team.Members.PageInfo != nil {
		users.PageInfo.HasNextPage = resp.Team.Members.PageInfo.HasNextPage
		if resp.Team.Members.PageInfo.EndCursor != nil {
			users.PageInfo.EndCursor = *resp.Team.Members.PageInfo.EndCursor
		}
	}

	return users
}

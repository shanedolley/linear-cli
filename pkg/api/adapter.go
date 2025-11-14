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
func (c *Client) GetIssuesNew(ctx context.Context, filter map[string]interface{}, first int, after string, orderBy string) (*Issues, error) {
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
func (c *Client) GetIssueNew(ctx context.Context, id string) (*Issue, error) {
	// Call generated function
	resp, err := GetIssue(ctx, c, id)
	if err != nil {
		return nil, err
	}

	// Convert generated response to legacy Issue type
	return convertToLegacyIssue(resp), nil
}

// IssueSearchNew wraps the generated SearchIssues function
func (c *Client) IssueSearchNew(ctx context.Context, term string, filter map[string]interface{}, first int, after string, orderBy string, includeArchived bool) (*Issues, error) {
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
func (c *Client) CreateIssueNew(ctx context.Context, input map[string]interface{}) (*Issue, error) {
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
func (c *Client) UpdateIssueNew(ctx context.Context, id string, input map[string]interface{}) (*Issue, error) {
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

package api

import (
	"encoding/json"
	"time"
)

// User represents a Linear user
type User struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	AvatarURL   string     `json:"avatarUrl"`
	DisplayName string     `json:"displayName"`
	IsMe        bool       `json:"isMe"`
	Active      bool       `json:"active"`
	Admin       bool       `json:"admin"`
	CreatedAt   *time.Time `json:"createdAt"`
}

// Team represents a Linear team
type Team struct {
	ID                 string  `json:"id"`
	Key                string  `json:"key"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Icon               *string `json:"icon"`
	Color              string  `json:"color"`
	Private            bool    `json:"private"`
	IssueCount         int     `json:"issueCount"`
	CyclesEnabled      bool    `json:"cyclesEnabled"`
	CycleStartDay      int     `json:"cycleStartDay"`
	CycleDuration      int     `json:"cycleDuration"`
	UpcomingCycleCount int     `json:"upcomingCycleCount"`
	States             []State `json:"states"` // Workflow states for this team
}

// Issue represents a Linear issue
type Issue struct {
	ID                  string       `json:"id"`
	Identifier          string       `json:"identifier"`
	Title               string       `json:"title"`
	Description         string       `json:"description"`
	Priority            int          `json:"priority"`
	Estimate            *float64     `json:"estimate"`
	CreatedAt           time.Time    `json:"createdAt"`
	UpdatedAt           time.Time    `json:"updatedAt"`
	DueDate             *string      `json:"dueDate"`
	State               *State       `json:"state"`
	Assignee            *User        `json:"assignee"`
	Team                *Team        `json:"team"`
	Labels              *Labels      `json:"labels"`
	Children            *Issues      `json:"children"`
	Parent              *Issue       `json:"parent"`
	URL                 string       `json:"url"`
	BranchName          string       `json:"branchName"`
	Cycle               *Cycle       `json:"cycle"`
	Project             *Project     `json:"project"`
	Attachments         *Attachments `json:"attachments"`
	Comments            *Comments    `json:"comments"`
	SnoozedUntilAt      *time.Time   `json:"snoozedUntilAt"`
	CompletedAt         *time.Time   `json:"completedAt"`
	CanceledAt          *time.Time   `json:"canceledAt"`
	ArchivedAt          *time.Time   `json:"archivedAt"`
	TriagedAt           *time.Time   `json:"triagedAt"`
	CustomerTicketCount int          `json:"customerTicketCount"`
	PreviousIdentifiers []string     `json:"previousIdentifiers"`
	// Additional fields
	Number                int              `json:"number"`
	BoardOrder            float64          `json:"boardOrder"`
	SubIssueSortOrder     float64          `json:"subIssueSortOrder"`
	PriorityLabel         string           `json:"priorityLabel"`
	IntegrationSourceType *string          `json:"integrationSourceType"`
	Creator               *User            `json:"creator"`
	Subscribers           *Users           `json:"subscribers"`
	Relations             *IssueRelations  `json:"relations"`
	History               *IssueHistory    `json:"history"`
	Reactions             []Reaction       `json:"reactions"`
	SlackIssueComments    []SlackComment   `json:"slackIssueComments"`
	ExternalUserCreator   *ExternalUser    `json:"externalUserCreator"`
	CustomerTickets       []CustomerTicket `json:"customerTickets"`
}

// State represents an issue state
type State struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
	Position    float64 `json:"position"`
}

// Project represents a Linear project
type Project struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	State       string     `json:"state"`
	Progress    float64    `json:"progress"`
	StartDate   *string    `json:"startDate"`
	TargetDate  *string    `json:"targetDate"`
	Lead        *User      `json:"lead"`
	Teams       *Teams     `json:"teams"`
	URL         string     `json:"url"`
	Icon        *string    `json:"icon"`
	Color       string     `json:"color"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt"`
	CanceledAt  *time.Time `json:"canceledAt"`
	ArchivedAt  *time.Time `json:"archivedAt"`
	Creator     *User      `json:"creator"`
	Members     *Users     `json:"members"`
	Issues      *Issues    `json:"issues"`
	// Additional fields
	SlugId              string          `json:"slugId"`
	Content             string          `json:"content"`
	ConvertedFromIssue  *Issue          `json:"convertedFromIssue"`
	LastAppliedTemplate *Template       `json:"lastAppliedTemplate"`
	ProjectUpdates      *ProjectUpdates `json:"projectUpdates"`
	Documents           *Documents      `json:"documents"`
	Health              string          `json:"health"`
	Scope               int             `json:"scope"`
	SlackNewIssue       bool            `json:"slackNewIssue"`
	SlackIssueComments  bool            `json:"slackIssueComments"`
	SlackIssueStatuses  bool            `json:"slackIssueStatuses"`
}

// Paginated collections
type Issues struct {
	Nodes    []Issue  `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

type Teams struct {
	Nodes    []Team   `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

type Projects struct {
	Nodes    []Project `json:"nodes"`
	PageInfo PageInfo  `json:"pageInfo"`
}

type Users struct {
	Nodes    []User   `json:"nodes"`
	PageInfo PageInfo `json:"pageInfo"`
}

type Labels struct {
	Nodes []Label `json:"nodes"`
}

type Label struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
	Parent      *Label  `json:"parent"`
}

// Cycle represents a Linear cycle (sprint)
type Cycle struct {
	ID           string     `json:"id"`
	Number       int        `json:"number"`
	Name         string     `json:"name"`
	Description  *string    `json:"description"`
	StartsAt     string     `json:"startsAt"`
	EndsAt       string     `json:"endsAt"`
	Progress     float64    `json:"progress"`
	CompletedAt  *time.Time `json:"completedAt"`
	ScopeHistory []float64  `json:"scopeHistory"`
}

// Attachment represents a file attachment or link
type Attachment struct {
	ID        string                 `json:"id"`
	Title     string                 `json:"title"`
	Subtitle  *string                `json:"subtitle"`
	URL       string                 `json:"url"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"createdAt"`
	Creator   *User                  `json:"creator"`

	// Use a map to capture any extra fields Linear might return
	Extra map[string]interface{} `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling to handle unexpected fields from Linear API
func (a *Attachment) UnmarshalJSON(data []byte) error {
	// Create an alias to avoid infinite recursion
	type Alias Attachment
	aux := &struct {
		*Alias
		// Capture extra fields that might come from Linear
		Source     interface{} `json:"source,omitempty"`
		SourceType interface{} `json:"sourceType,omitempty"`
	}{
		Alias: (*Alias)(a),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Store unexpected fields in Extra map if needed
	if aux.Source != nil || aux.SourceType != nil {
		a.Extra = make(map[string]interface{})
		if aux.Source != nil {
			a.Extra["source"] = aux.Source
		}
		if aux.SourceType != nil {
			a.Extra["sourceType"] = aux.SourceType
		}
	}

	return nil
}

// Attachments represents a paginated list of attachments
type Attachments struct {
	Nodes []Attachment `json:"nodes"`
}

// Initiative represents a Linear initiative
type Initiative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// Additional types for expanded fields
type IssueRelations struct {
	Nodes []IssueRelation `json:"nodes"`
}

type IssueRelation struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Issue        *Issue `json:"issue"`
	RelatedIssue *Issue `json:"relatedIssue"`
}

type IssueHistory struct {
	Nodes []IssueHistoryEntry `json:"nodes"`
}

type IssueHistoryEntry struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Changes         string    `json:"changes"`
	Actor           *User     `json:"actor"`
	FromAssignee    *User     `json:"fromAssignee"`
	ToAssignee      *User     `json:"toAssignee"`
	FromState       *State    `json:"fromState"`
	ToState         *State    `json:"toState"`
	FromPriority    *int      `json:"fromPriority"`
	ToPriority      *int      `json:"toPriority"`
	FromTitle       *string   `json:"fromTitle"`
	ToTitle         *string   `json:"toTitle"`
	FromCycle       *Cycle    `json:"fromCycle"`
	ToCycle         *Cycle    `json:"toCycle"`
	FromProject     *Project  `json:"fromProject"`
	ToProject       *Project  `json:"toProject"`
	AddedLabelIds   []string  `json:"addedLabelIds"`
	RemovedLabelIds []string  `json:"removedLabelIds"`
}

type Reaction struct {
	ID        string    `json:"id"`
	Emoji     string    `json:"emoji"`
	User      *User     `json:"user"`
	CreatedAt time.Time `json:"createdAt"`
}

type SlackComment struct {
	ID   string `json:"id"`
	Body string `json:"body"`
}

type ExternalUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CustomerTicket struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	CreatedAt  time.Time `json:"createdAt"`
	ExternalId string    `json:"externalId"`
}

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Milestone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TargetDate  *string   `json:"targetDate"`
	Projects    *Projects `json:"projects"`
}

type Roadmaps struct {
	Nodes []Roadmap `json:"nodes"`
}

type Roadmap struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Creator     *User     `json:"creator"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ProjectUpdates struct {
	Nodes []ProjectUpdate `json:"nodes"`
}

type ProjectUpdate struct {
	ID        string     `json:"id"`
	Body      string     `json:"body"`
	User      *User      `json:"user"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	EditedAt  *time.Time `json:"editedAt"`
	Health    string     `json:"health"`
}

type Documents struct {
	Nodes []Document `json:"nodes"`
}

type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Icon      *string   `json:"icon"`
	Color     string    `json:"color"`
	Creator   *User     `json:"creator"`
	UpdatedBy *User     `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ProjectLinks struct {
	Nodes []ProjectLink `json:"nodes"`
}

type ProjectLink struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Label     string    `json:"label"`
	Creator   *User     `json:"creator"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// GetViewer returns the current authenticated user

// GetIssues returns a list of issues with optional filtering

// IssueSearch returns issues that match a full-text query

// GetIssue returns a single issue by ID

// GetTeams returns a list of teams

// GetProjects returns a list of projects

// GetProject returns a single project by ID

// UpdateIssue updates an issue's fields

// CreateIssue creates a new issue

// GetTeam returns a single team by key

// Comment represents a Linear comment
type Comment struct {
	ID        string     `json:"id"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	EditedAt  *time.Time `json:"editedAt"`
	User      *User      `json:"user"`
	Parent    *Comment   `json:"parent"`
	Children  *Comments  `json:"children"`
}

// Comments represents a paginated list of comments
type Comments struct {
	Nodes    []Comment `json:"nodes"`
	PageInfo PageInfo  `json:"pageInfo"`
}

// WorkflowState represents a Linear workflow state
type WorkflowState struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Color       string  `json:"color"`
	Description string  `json:"description"`
	Position    float64 `json:"position"`
}

// GetTeamStates returns workflow states for a team

// GetTeamMembers returns members of a specific team

// GetUsers returns a list of all users

// GetUser returns a specific user by email

// GetIssueComments returns comments for a specific issue

// CreateComment creates a new comment on an issue

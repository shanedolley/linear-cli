package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
)

// mockGraphQLClient implements graphql.Client for hermetic, offline unit
// tests. It returns canned JSON responses keyed by GraphQL operation name
// and records how many times each operation was invoked so tests can
// assert on caching behavior (i.e. that a second resolve call does not
// re-hit the "network").
type mockGraphQLClient struct {
	// responses maps operation name -> raw JSON to unmarshal into resp.Data.
	responses map[string]string
	// errors maps operation name -> error to return instead of a response.
	errors map[string]error
	// calls records the operation name of every MakeRequest invocation, in order.
	calls []string
}

func newMockGraphQLClient() *mockGraphQLClient {
	return &mockGraphQLClient{
		responses: make(map[string]string),
		errors:    make(map[string]error),
	}
}

func (m *mockGraphQLClient) MakeRequest(_ context.Context, req *graphql.Request, resp *graphql.Response) error {
	m.calls = append(m.calls, req.OpName)

	if err, ok := m.errors[req.OpName]; ok {
		return err
	}

	raw, ok := m.responses[req.OpName]
	if !ok {
		return &mockUnhandledOperationError{opName: req.OpName}
	}

	return json.Unmarshal([]byte(raw), resp.Data)
}

// callCount returns how many times the given operation was invoked.
func (m *mockGraphQLClient) callCount(opName string) int {
	count := 0
	for _, c := range m.calls {
		if c == opName {
			count++
		}
	}
	return count
}

type mockUnhandledOperationError struct {
	opName string
}

func (e *mockUnhandledOperationError) Error() string {
	return "mockGraphQLClient: no canned response registered for operation " + e.opName
}

// --- isUUID ---

func TestIsUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid uppercase uuid", "550E8400-E29B-41D4-A716-446655440000", true},
		{"valid mixed-case uuid", "550e8400-E29b-41D4-a716-446655440000", true},
		{"team key", "ENG", false},
		{"lowercase team key", "eng", false},
		{"email", "jane@example.com", false},
		{"name", "Jane Doe", false},
		{"empty string", "", false},
		{"sentinel me", "me", false},
		{"sentinel unassigned", "unassigned", false},
		{"sentinel none", "none", false},
		{"missing hyphens", "550e8400e29b41d4a716446655440000", false},
		{"too short", "550e8400-e29b-41d4-a716-44665544000", false},
		{"too long", "550e8400-e29b-41d4-a716-4466554400000", false},
		{"non-hex characters", "550e8400-e29b-41d4-a716-44665544000g", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUUID(tt.input); got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- ResolverCache ---

func TestNewResolverCacheIsEmpty(t *testing.T) {
	cache := newResolverCache()
	if cache == nil {
		t.Fatal("newResolverCache() returned nil")
	}
	if len(cache.teams) != 0 || len(cache.users) != 0 || len(cache.projects) != 0 || len(cache.initiatives) != 0 || len(cache.states) != 0 {
		t.Fatal("newResolverCache() should return a cache with empty maps")
	}
}

// --- resolveTeam ---

func TestResolveTeam_UUIDPassthrough(t *testing.T) {
	client := newMockGraphQLClient()
	cache := newResolverCache()

	id := "550e8400-e29b-41d4-a716-446655440000"
	got, err := resolveTeam(context.Background(), client, cache, id)
	if err != nil {
		t.Fatalf("resolveTeam() error = %v", err)
	}
	if got != id {
		t.Errorf("resolveTeam() = %q, want %q", got, id)
	}
	if len(client.calls) != 0 {
		t.Errorf("expected no API calls for UUID passthrough, got %v", client.calls)
	}
}

func TestResolveTeam_ExactMatch(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetTeam"] = `{"team": {"id": "team-uuid-1", "key": "ENG", "name": "Engineering", "description": null, "icon": null, "color": null, "private": false, "issueCount": 10, "cyclesEnabled": false, "cycleStartDay": 0, "cycleDuration": 0, "upcomingCycleCount": 0}}`
	cache := newResolverCache()

	got, err := resolveTeam(context.Background(), client, cache, "eng")
	if err != nil {
		t.Fatalf("resolveTeam() error = %v", err)
	}
	if got != "team-uuid-1" {
		t.Errorf("resolveTeam() = %q, want %q", got, "team-uuid-1")
	}
	if client.callCount("GetTeam") != 1 {
		t.Errorf("expected exactly 1 GetTeam call, got %d", client.callCount("GetTeam"))
	}
}

func TestResolveTeam_CacheHit(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetTeam"] = `{"team": {"id": "team-uuid-1", "key": "ENG", "name": "Engineering", "description": null, "icon": null, "color": null, "private": false, "issueCount": 10, "cyclesEnabled": false, "cycleStartDay": 0, "cycleDuration": 0, "upcomingCycleCount": 0}}`
	cache := newResolverCache()

	if _, err := resolveTeam(context.Background(), client, cache, "ENG"); err != nil {
		t.Fatalf("first resolveTeam() error = %v", err)
	}
	if client.callCount("GetTeam") != 1 {
		t.Fatalf("expected exactly 1 GetTeam call after first resolve, got %d", client.callCount("GetTeam"))
	}

	// Second call, different case, should hit the cache and make no further API calls.
	got, err := resolveTeam(context.Background(), client, cache, "eng")
	if err != nil {
		t.Fatalf("second resolveTeam() error = %v", err)
	}
	if got != "team-uuid-1" {
		t.Errorf("resolveTeam() = %q, want %q", got, "team-uuid-1")
	}
	if client.callCount("GetTeam") != 1 {
		t.Errorf("expected cache hit to avoid a second GetTeam call, total calls = %d", client.callCount("GetTeam"))
	}
}

func TestResolveTeam_NotFound(t *testing.T) {
	client := newMockGraphQLClient()
	client.errors["GetTeam"] = &mockUnhandledOperationError{opName: "team not found"}
	client.responses["ListTeams"] = `{"teams": {"nodes": [{"id": "team-uuid-2", "key": "OPS", "name": "Operations", "description": null, "private": false, "issueCount": 3}], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	_, err := resolveTeam(context.Background(), client, cache, "MISSING")
	if err == nil {
		t.Fatal("expected error for unknown team key, got nil")
	}
	if !strings.Contains(err.Error(), "MISSING") {
		t.Errorf("expected error to mention the missing key, got: %v", err)
	}
}

// --- resolveUser ---

func TestResolveUser_UUIDPassthrough(t *testing.T) {
	client := newMockGraphQLClient()
	cache := newResolverCache()

	id := "550e8400-e29b-41d4-a716-446655440000"
	got, err := resolveUser(context.Background(), client, cache, id)
	if err != nil {
		t.Fatalf("resolveUser() error = %v", err)
	}
	if got != id {
		t.Errorf("resolveUser() = %q, want %q", got, id)
	}
	if len(client.calls) != 0 {
		t.Errorf("expected no API calls for UUID passthrough, got %v", client.calls)
	}
}

func TestResolveUser_Me(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetViewer"] = `{"viewer": {"id": "user-uuid-me", "name": "Shane Dolley", "email": "shane@example.com", "avatarUrl": null, "displayName": "Shane", "isMe": true, "active": true, "admin": true, "createdAt": "2024-01-01T00:00:00Z"}}`
	cache := newResolverCache()

	got, err := resolveUser(context.Background(), client, cache, "me")
	if err != nil {
		t.Fatalf("resolveUser() error = %v", err)
	}
	if got != "user-uuid-me" {
		t.Errorf("resolveUser() = %q, want %q", got, "user-uuid-me")
	}
}

func TestResolveUser_ExactMatch(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetUserByEmail"] = `{"users": {"nodes": [{"id": "user-uuid-1", "name": "Jane Doe", "email": "jane@example.com", "avatarUrl": null, "displayName": "Jane", "isMe": false, "active": true, "admin": false, "createdAt": "2024-01-01T00:00:00Z"}]}}`
	cache := newResolverCache()

	got, err := resolveUser(context.Background(), client, cache, "jane@example.com")
	if err != nil {
		t.Fatalf("resolveUser() error = %v", err)
	}
	if got != "user-uuid-1" {
		t.Errorf("resolveUser() = %q, want %q", got, "user-uuid-1")
	}
	if client.callCount("GetUserByEmail") != 1 {
		t.Errorf("expected exactly 1 GetUserByEmail call, got %d", client.callCount("GetUserByEmail"))
	}
}

func TestResolveUser_Ambiguous(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetUserByEmail"] = `{"users": {"nodes": [
		{"id": "user-uuid-1", "name": "Jane Doe", "email": "jane@example.com", "avatarUrl": null, "displayName": "Jane", "isMe": false, "active": true, "admin": false, "createdAt": "2024-01-01T00:00:00Z"},
		{"id": "user-uuid-2", "name": "Jane Doe", "email": "jane.doe@example.com", "avatarUrl": null, "displayName": "Jane D", "isMe": false, "active": true, "admin": false, "createdAt": "2024-01-01T00:00:00Z"}
	]}}`
	cache := newResolverCache()

	_, err := resolveUser(context.Background(), client, cache, "Jane Doe")
	if err == nil {
		t.Fatal("expected an ambiguous match error, got nil")
	}
	if !strings.Contains(err.Error(), "Multiple matches") {
		t.Errorf("expected ambiguous error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "jane@example.com") || !strings.Contains(err.Error(), "jane.doe@example.com") {
		t.Errorf("expected error to list both candidate emails, got: %v", err)
	}
}

func TestResolveUser_NotFound(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetUserByEmail"] = `{"users": {"nodes": []}}`
	cache := newResolverCache()

	_, err := resolveUser(context.Background(), client, cache, "ghost@example.com")
	if err == nil {
		t.Fatal("expected error for unknown user, got nil")
	}
	if !strings.Contains(err.Error(), "ghost@example.com") {
		t.Errorf("expected error to mention the user reference, got: %v", err)
	}
}

func TestResolveUser_CacheHit(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetUserByEmail"] = `{"users": {"nodes": [{"id": "user-uuid-1", "name": "Jane Doe", "email": "jane@example.com", "avatarUrl": null, "displayName": "Jane", "isMe": false, "active": true, "admin": false, "createdAt": "2024-01-01T00:00:00Z"}]}}`
	cache := newResolverCache()

	if _, err := resolveUser(context.Background(), client, cache, "jane@example.com"); err != nil {
		t.Fatalf("first resolveUser() error = %v", err)
	}
	if client.callCount("GetUserByEmail") != 1 {
		t.Fatalf("expected exactly 1 GetUserByEmail call after first resolve, got %d", client.callCount("GetUserByEmail"))
	}

	got, err := resolveUser(context.Background(), client, cache, "jane@example.com")
	if err != nil {
		t.Fatalf("second resolveUser() error = %v", err)
	}
	if got != "user-uuid-1" {
		t.Errorf("resolveUser() = %q, want %q", got, "user-uuid-1")
	}
	if client.callCount("GetUserByEmail") != 1 {
		t.Errorf("expected cache hit to avoid a second GetUserByEmail call, total calls = %d", client.callCount("GetUserByEmail"))
	}
}

// --- resolveProject ---

func TestResolveProject_UUIDPassthrough(t *testing.T) {
	client := newMockGraphQLClient()
	cache := newResolverCache()

	id := "550e8400-e29b-41d4-a716-446655440000"
	got, err := resolveProject(context.Background(), client, cache, id)
	if err != nil {
		t.Fatalf("resolveProject() error = %v", err)
	}
	if got != id {
		t.Errorf("resolveProject() = %q, want %q", got, id)
	}
	if len(client.calls) != 0 {
		t.Errorf("expected no API calls for UUID passthrough, got %v", client.calls)
	}
}

func TestResolveProject_ExactMatch(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListProjects"] = `{"projects": {"nodes": [{"id": "project-uuid-1", "name": "Q1 Roadmap", "description": "", "state": "started", "progress": 0.5, "startDate": null, "targetDate": null, "url": "https://linear.app/p/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "lead": null, "teams": {"nodes": []}}], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	got, err := resolveProject(context.Background(), client, cache, "q1 roadmap")
	if err != nil {
		t.Fatalf("resolveProject() error = %v", err)
	}
	if got != "project-uuid-1" {
		t.Errorf("resolveProject() = %q, want %q", got, "project-uuid-1")
	}
	if client.callCount("ListProjects") != 1 {
		t.Errorf("expected exactly 1 ListProjects call, got %d", client.callCount("ListProjects"))
	}
}

func TestResolveProject_Ambiguous(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListProjects"] = `{"projects": {"nodes": [
		{"id": "project-uuid-1", "name": "Roadmap", "description": "", "state": "started", "progress": 0.5, "startDate": null, "targetDate": null, "url": "https://linear.app/p/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "lead": null, "teams": {"nodes": []}},
		{"id": "project-uuid-2", "name": "Roadmap", "description": "", "state": "started", "progress": 0.1, "startDate": null, "targetDate": null, "url": "https://linear.app/p/2", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "lead": null, "teams": {"nodes": []}}
	], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	_, err := resolveProject(context.Background(), client, cache, "Roadmap")
	if err == nil {
		t.Fatal("expected an ambiguous match error, got nil")
	}
	if !strings.Contains(err.Error(), "Multiple matches") {
		t.Errorf("expected ambiguous error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "project-uuid-1") || !strings.Contains(err.Error(), "project-uuid-2") {
		t.Errorf("expected error to list both candidate ids, got: %v", err)
	}
}

func TestResolveProject_NotFound(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListProjects"] = `{"projects": {"nodes": [], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	_, err := resolveProject(context.Background(), client, cache, "Nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("expected error to mention the project name, got: %v", err)
	}
}

func TestResolveProject_CacheHit(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListProjects"] = `{"projects": {"nodes": [{"id": "project-uuid-1", "name": "Q1 Roadmap", "description": "", "state": "started", "progress": 0.5, "startDate": null, "targetDate": null, "url": "https://linear.app/p/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "lead": null, "teams": {"nodes": []}}], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	if _, err := resolveProject(context.Background(), client, cache, "Q1 Roadmap"); err != nil {
		t.Fatalf("first resolveProject() error = %v", err)
	}
	if client.callCount("ListProjects") != 1 {
		t.Fatalf("expected exactly 1 ListProjects call after first resolve, got %d", client.callCount("ListProjects"))
	}

	got, err := resolveProject(context.Background(), client, cache, "q1 roadmap")
	if err != nil {
		t.Fatalf("second resolveProject() error = %v", err)
	}
	if got != "project-uuid-1" {
		t.Errorf("resolveProject() = %q, want %q", got, "project-uuid-1")
	}
	if client.callCount("ListProjects") != 1 {
		t.Errorf("expected cache hit to avoid a second ListProjects call, total calls = %d", client.callCount("ListProjects"))
	}
}

// --- resolveInitiative ---

func TestResolveInitiative_UUIDPassthrough(t *testing.T) {
	client := newMockGraphQLClient()
	cache := newResolverCache()

	id := "550e8400-e29b-41d4-a716-446655440000"
	got, err := resolveInitiative(context.Background(), client, cache, id)
	if err != nil {
		t.Fatalf("resolveInitiative() error = %v", err)
	}
	if got != id {
		t.Errorf("resolveInitiative() = %q, want %q", got, id)
	}
	if len(client.calls) != 0 {
		t.Errorf("expected no API calls for UUID passthrough, got %v", client.calls)
	}
}

func TestResolveInitiative_ExactMatch(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListInitiatives"] = `{"initiatives": {"nodes": [{"id": "initiative-uuid-1", "name": "Platform Modernization", "status": "Active", "health": null, "icon": null, "color": null, "targetDate": null, "url": "https://linear.app/i/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "owner": null}], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	got, err := resolveInitiative(context.Background(), client, cache, "platform modernization")
	if err != nil {
		t.Fatalf("resolveInitiative() error = %v", err)
	}
	if got != "initiative-uuid-1" {
		t.Errorf("resolveInitiative() = %q, want %q", got, "initiative-uuid-1")
	}
	if client.callCount("ListInitiatives") != 1 {
		t.Errorf("expected exactly 1 ListInitiatives call, got %d", client.callCount("ListInitiatives"))
	}
}

func TestResolveInitiative_Ambiguous(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListInitiatives"] = `{"initiatives": {"nodes": [
		{"id": "initiative-uuid-1", "name": "Growth", "status": "Active", "health": null, "icon": null, "color": null, "targetDate": null, "url": "https://linear.app/i/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "owner": null},
		{"id": "initiative-uuid-2", "name": "Growth", "status": "Planned", "health": null, "icon": null, "color": null, "targetDate": null, "url": "https://linear.app/i/2", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "owner": null}
	], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	_, err := resolveInitiative(context.Background(), client, cache, "Growth")
	if err == nil {
		t.Fatal("expected an ambiguous match error, got nil")
	}
	if !strings.Contains(err.Error(), "Multiple matches") {
		t.Errorf("expected ambiguous error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "initiative-uuid-1") || !strings.Contains(err.Error(), "initiative-uuid-2") {
		t.Errorf("expected error to list both candidate ids, got: %v", err)
	}
}

func TestResolveInitiative_NotFound(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListInitiatives"] = `{"initiatives": {"nodes": [], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	_, err := resolveInitiative(context.Background(), client, cache, "Nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown initiative, got nil")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("expected error to mention the initiative name, got: %v", err)
	}
}

func TestResolveInitiative_CacheHit(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["ListInitiatives"] = `{"initiatives": {"nodes": [{"id": "initiative-uuid-1", "name": "Platform Modernization", "status": "Active", "health": null, "icon": null, "color": null, "targetDate": null, "url": "https://linear.app/i/1", "createdAt": "2024-01-01T00:00:00Z", "updatedAt": "2024-01-01T00:00:00Z", "owner": null}], "pageInfo": {"hasNextPage": false, "endCursor": null}}}`
	cache := newResolverCache()

	if _, err := resolveInitiative(context.Background(), client, cache, "Platform Modernization"); err != nil {
		t.Fatalf("first resolveInitiative() error = %v", err)
	}
	if client.callCount("ListInitiatives") != 1 {
		t.Fatalf("expected exactly 1 ListInitiatives call after first resolve, got %d", client.callCount("ListInitiatives"))
	}

	got, err := resolveInitiative(context.Background(), client, cache, "platform modernization")
	if err != nil {
		t.Fatalf("second resolveInitiative() error = %v", err)
	}
	if got != "initiative-uuid-1" {
		t.Errorf("resolveInitiative() = %q, want %q", got, "initiative-uuid-1")
	}
	if client.callCount("ListInitiatives") != 1 {
		t.Errorf("expected cache hit to avoid a second ListInitiatives call, total calls = %d", client.callCount("ListInitiatives"))
	}
}

// --- resolveWorkflowState ---

func TestResolveWorkflowState_UUIDPassthrough(t *testing.T) {
	client := newMockGraphQLClient()
	cache := newResolverCache()

	id := "550e8400-e29b-41d4-a716-446655440000"
	got, err := resolveWorkflowState(context.Background(), client, cache, "ENG", id)
	if err != nil {
		t.Fatalf("resolveWorkflowState() error = %v", err)
	}
	if got != id {
		t.Errorf("resolveWorkflowState() = %q, want %q", got, id)
	}
	if len(client.calls) != 0 {
		t.Errorf("expected no API calls for UUID passthrough, got %v", client.calls)
	}
}

func TestResolveWorkflowState_ExactMatch(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetTeamStates"] = `{"team": {"states": {"nodes": [
		{"id": "state-uuid-1", "name": "Todo", "type": "unstarted", "color": "#000", "description": null, "position": 0},
		{"id": "state-uuid-2", "name": "In Progress", "type": "started", "color": "#000", "description": null, "position": 1},
		{"id": "state-uuid-3", "name": "Done", "type": "completed", "color": "#000", "description": null, "position": 2}
	]}}}`
	cache := newResolverCache()

	got, err := resolveWorkflowState(context.Background(), client, cache, "ENG", "in progress")
	if err != nil {
		t.Fatalf("resolveWorkflowState() error = %v", err)
	}
	if got != "state-uuid-2" {
		t.Errorf("resolveWorkflowState() = %q, want %q", got, "state-uuid-2")
	}
	if client.callCount("GetTeamStates") != 1 {
		t.Errorf("expected exactly 1 GetTeamStates call, got %d", client.callCount("GetTeamStates"))
	}
}

func TestResolveWorkflowState_NotFound(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetTeamStates"] = `{"team": {"states": {"nodes": [
		{"id": "state-uuid-1", "name": "Todo", "type": "unstarted", "color": "#000", "description": null, "position": 0}
	]}}}`
	cache := newResolverCache()

	_, err := resolveWorkflowState(context.Background(), client, cache, "ENG", "Nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown state name, got nil")
	}
	if !strings.Contains(err.Error(), "Nonexistent") {
		t.Errorf("expected error to mention the state name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Todo") {
		t.Errorf("expected error to list the valid state names, got: %v", err)
	}
}

func TestResolveWorkflowState_CacheHit(t *testing.T) {
	client := newMockGraphQLClient()
	client.responses["GetTeamStates"] = `{"team": {"states": {"nodes": [
		{"id": "state-uuid-1", "name": "Todo", "type": "unstarted", "color": "#000", "description": null, "position": 0}
	]}}}`
	cache := newResolverCache()

	if _, err := resolveWorkflowState(context.Background(), client, cache, "ENG", "Todo"); err != nil {
		t.Fatalf("first resolveWorkflowState() error = %v", err)
	}
	if client.callCount("GetTeamStates") != 1 {
		t.Fatalf("expected exactly 1 GetTeamStates call after first resolve, got %d", client.callCount("GetTeamStates"))
	}

	got, err := resolveWorkflowState(context.Background(), client, cache, "ENG", "todo")
	if err != nil {
		t.Fatalf("second resolveWorkflowState() error = %v", err)
	}
	if got != "state-uuid-1" {
		t.Errorf("resolveWorkflowState() = %q, want %q", got, "state-uuid-1")
	}
	if client.callCount("GetTeamStates") != 1 {
		t.Errorf("expected cache hit to avoid a second GetTeamStates call, total calls = %d", client.callCount("GetTeamStates"))
	}
}

// Sanity check that our mock actually satisfies genqlient's graphql.Client
// interface, since that's what makes it usable with the generated api.* functions.
var _ graphql.Client = (*mockGraphQLClient)(nil)

// Sanity: make sure api package is referenced so goimports/staticcheck don't
// flag an unused import if a future refactor trims direct api.* usage here.
var _ = api.NullSentinel

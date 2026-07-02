package cmd

import (
	"errors"
	"testing"

	"github.com/Khan/genqlient/graphql"
)

// withMockClient swaps the package's client factory for one that returns the
// given mock, and restores the original when the test ends. This is the seam
// that makes RunE handlers testable offline: before the RunE migration a
// handler built its own client from disk auth and called os.Exit on failure,
// so it could only be exercised by the live smoke suite.
func withMockClient(t *testing.T, mock graphql.Client) {
	t.Helper()
	orig := newGraphQLClient
	newGraphQLClient = func() (graphql.Client, error) { return mock, nil }
	t.Cleanup(func() { newGraphQLClient = orig })
}

// A handler now returns its error (RunE) instead of calling os.Exit, and talks
// to the injected client instead of the network. Both are asserted here.
func TestStateGetHandler_ReturnsErrorFromInjectedClient(t *testing.T) {
	mock := newMockGraphQLClient()
	mock.errors["GetWorkflowState"] = errors.New("boom")
	withMockClient(t, mock)

	err := stateGetCmd.RunE(stateGetCmd, []string{"STATE-1"})
	if err == nil {
		t.Fatal("expected the handler to return an error, got nil")
	}
	if mock.callCount("GetWorkflowState") != 1 {
		t.Errorf("handler did not call the injected client: GetWorkflowState count = %d, want 1", mock.callCount("GetWorkflowState"))
	}
}

// A null WorkflowState (a reference that matches nothing) must produce a clean
// "not found" error rather than a nil-pointer panic, and it must not exit.
func TestStateGetHandler_NotFoundIsCleanError(t *testing.T) {
	mock := newMockGraphQLClient()
	mock.responses["GetWorkflowState"] = `{"workflowState": null}`
	withMockClient(t, mock)

	err := stateGetCmd.RunE(stateGetCmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected a not-found error, got nil")
	}
}

package cmd

import (
	"errors"

	"github.com/Khan/genqlient/graphql"
	"github.com/shanedolley/lincli/pkg/api"
	"github.com/shanedolley/lincli/pkg/auth"
)

// errNotAuthenticated is returned when no API key is configured. A RunE handler
// surfaces it through the central error formatter in Execute, which renders it
// in the active output mode.
var errNotAuthenticated = errors.New("Not authenticated. Run 'lincli auth' first.")

// errSilent tells Execute to exit non-zero without printing anything more. A
// handler that renders its own error output (rather than deferring to the
// central formatter) returns errSilent so the message is not printed twice.
var errSilent = errors.New("")

// newGraphQLClient builds an authenticated Linear client. It is a package
// variable, not a plain function, so a test can substitute a mock
// graphql.Client. That injection point is what makes the command handlers
// unit-testable without reaching the network.
var newGraphQLClient = func() (graphql.Client, error) {
	authHeader, err := auth.GetAuthHeader()
	if err != nil {
		return nil, errNotAuthenticated
	}
	return api.NewClient(authHeader), nil
}

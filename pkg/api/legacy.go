package api

import (
	"context"
)

// This file contains legacy hand-written GraphQL queries that haven't been migrated
// to genqlient yet. These will be migrated in future iterations.

// GetTeamStates returns workflow states for a team
func (c *Client) GetTeamStates(ctx context.Context, teamKey string) ([]WorkflowState, error) {
	query := `
		query TeamStates($key: String!) {
			team(id: $key) {
				states {
					nodes {
						id
						name
						type
						color
						description
						position
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"key": teamKey,
	}

	var response struct {
		Team struct {
			States struct {
				Nodes []WorkflowState `json:"nodes"`
			} `json:"states"`
		} `json:"team"`
	}

	err := c.Execute(ctx, query, variables, &response)
	if err != nil {
		return nil, err
	}

	return response.Team.States.Nodes, nil
}

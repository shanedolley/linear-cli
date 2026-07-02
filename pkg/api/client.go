package api

//go:generate go run github.com/Khan/genqlient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Khan/genqlient/graphql"
)

const (
	BaseURL = "https://api.linear.app/graphql"
)

type Client struct {
	httpClient *http.Client
	authHeader string
	baseURL    string
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message   string                 `json:"message"`
	Locations []GraphQLErrorLocation `json:"locations,omitempty"`
	Path      []interface{}          `json:"path,omitempty"`
}

type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewClient creates a new Linear API client
func NewClient(authHeader string) *Client {
	return NewClientWithURL(BaseURL, authHeader)
}

// NewClientWithURL creates a new Linear API client with custom URL
func NewClientWithURL(baseURL, authHeader string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authHeader: authHeader,
		baseURL:    baseURL,
	}
}

// Execute performs a GraphQL request
func (c *Client) Execute(ctx context.Context, query string, variables map[string]interface{}, result interface{}) error {
	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("User-Agent", "lincli/0.1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	if result != nil {
		if err := json.Unmarshal(gqlResp.Data, result); err != nil {
			return fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	return nil
}

// NullSentinel is a special value that will be converted to null in the GraphQL request.
// Use this when you need to explicitly send null to clear a field.
const NullSentinel = "__LINCLI_NULL__"

// stripNulls recursively removes null values from a map, and converts NullSentinel to null
func stripNulls(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if v == nil {
			continue
		}
		// Convert sentinel value to null
		if strVal, ok := v.(string); ok && strVal == NullSentinel {
			result[k] = nil
			continue
		}
		if innerMap, ok := v.(map[string]interface{}); ok {
			stripped := stripNulls(innerMap)
			if len(stripped) > 0 {
				result[k] = stripped
			}
		} else if innerSlice, ok := v.([]interface{}); ok {
			// Recurse into map elements so nested filter objects (for
			// example the "or"/"and" arrays) get their null comparator
			// fields stripped, the same way top-level filter fields are.
			// Without this, Linear treats the extra null fields as
			// constraints and the filter matches nothing. Non-map elements
			// (for example ID strings in memberIds) are kept as-is.
			strippedSlice := make([]interface{}, len(innerSlice))
			for i, item := range innerSlice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					strippedSlice[i] = stripNulls(itemMap)
				} else {
					strippedSlice[i] = item
				}
			}
			result[k] = strippedSlice
		} else {
			result[k] = v
		}
	}
	return result
}

// MakeRequest implements the graphql.Client interface required by genqlient
func (c *Client) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	// Build the GraphQL request body
	// We need to strip null values from variables because Linear's API doesn't like them
	type graphQLRequest struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables,omitempty"`
	}

	// Convert variables to map and strip nulls
	var variables map[string]interface{}
	if req.Variables != nil {
		varBytes, err := json.Marshal(req.Variables)
		if err != nil {
			return fmt.Errorf("failed to marshal variables: %w", err)
		}
		if err := json.Unmarshal(varBytes, &variables); err != nil {
			return fmt.Errorf("failed to unmarshal variables: %w", err)
		}
		// Strip null values recursively
		variables = stripNulls(variables)
	}

	reqBody := graphQLRequest{
		Query:     req.Query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// DEBUG: Print the request body for debugging
	if os.Getenv("LINCLI_DEBUG_GQL") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: GraphQL Request: %s\n", string(jsonBody))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.authHeader)
	httpReq.Header.Set("User-Agent", "lincli/0.1.0")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", httpResp.StatusCode, string(body))
	}

	// Parse into a GraphQL response structure
	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	// Unmarshal the data into the response Data field
	if resp.Data != nil {
		if err := json.Unmarshal(gqlResp.Data, resp.Data); err != nil {
			return fmt.Errorf("failed to unmarshal data: %w", err)
		}
	}

	return nil
}

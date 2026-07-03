package api

//go:generate go run github.com/Khan/genqlient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

// NullSentinel is an unguessable, per-process marker. Assigning it to an
// optional field asks stripNulls to send an explicit JSON null (to clear the
// field) rather than omitting the field. genqlient generates mutation-input
// fields as pointers without `omitempty`, so a nil pointer would serialize as
// `null`; stripNulls drops those to distinguish "absent" from "clear", and this
// sentinel is the way a command opts back in to sending an explicit null.
//
// The marker carries random bytes so that no user-supplied field value (an
// issue title, a comment body) can ever equal it and be silently turned into
// null. See docs/adr/0001-explicit-null-handling.md.
var NullSentinel = newNullSentinel()

func newNullSentinel() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand should never fail; if it somehow does, fall back to a
		// fixed marker. A collision with real Linear data remains unlikely.
		return "__LINCLI_NULL__"
	}
	return "__LINCLI_NULL__" + hex.EncodeToString(buf)
}

// stripNulls recursively removes null values from a map and converts
// NullSentinel to null. Empty nested objects are preserved (not dropped), so a
// caller can send an intentionally empty object; only Go nil values, which
// genqlient emits for unset optional fields, are removed.
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
			result[k] = stripNulls(innerMap)
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

// sensitiveVarSubstrings marks a request variable as secret-bearing when its
// normalized name contains any of these (for example a webhook signing secret).
// LINCLI_DEBUG_GQL redacts matching variables so the debug dump can be shared
// without leaking credentials. Matching is case-insensitive and ignores
// separators, so compound names (clientSecret, signingSecret, personalApiKey,
// api_key, accessToken) are covered, not just the bare word.
var sensitiveVarSubstrings = []string{"secret", "token", "password", "apikey", "credential"}

// isSensitiveVarKey reports whether a variable name looks secret-bearing.
func isSensitiveVarKey(k string) bool {
	norm := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(k))
	for _, s := range sensitiveVarSubstrings {
		if strings.Contains(norm, s) {
			return true
		}
	}
	return false
}

// redactSensitive returns a deep copy of v with the values of any
// sensitive-named keys replaced by "[REDACTED]". It walks nested maps and
// slices so secrets inside input objects are covered too.
func redactSensitive(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			if isSensitiveVarKey(k) {
				out[k] = "[REDACTED]"
				continue
			}
			out[k] = redactSensitive(child)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, child := range val {
			out[i] = redactSensitive(child)
		}
		return out
	default:
		return v
	}
}

// MakeRequest implements the graphql.Client interface required by genqlient
func (c *Client) MakeRequest(ctx context.Context, req *graphql.Request, resp *graphql.Response) error {
	// Build the GraphQL request body.
	// We strip null values from variables because Linear's API rejects them.

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

	reqBody := GraphQLRequest{
		Query:     req.Query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// DEBUG: Print the request body for debugging, with any secret-bearing
	// variables redacted so the dump is safe to share.
	if os.Getenv("LINCLI_DEBUG_GQL") != "" {
		debugBody := GraphQLRequest{Query: req.Query}
		if variables != nil {
			if redacted, ok := redactSensitive(variables).(map[string]interface{}); ok {
				debugBody.Variables = redacted
			}
		}
		if debugJSON, err := json.Marshal(debugBody); err == nil {
			fmt.Fprintf(os.Stderr, "DEBUG: GraphQL Request: %s\n", string(debugJSON))
		}
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

package linearapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/roeyazroel/linear-tui/internal/logger"
	"github.com/shurcooL/graphql"
)

// Cycle represents a Linear cycle (sprint).
type Cycle struct {
	ID                         string
	Name                       string
	Number                     int
	StartsAt                   time.Time
	EndsAt                     time.Time
	Progress                   float64 // 0.0–1.0
	TeamID                     string
	CompletedIssueCountHistory []int // Daily completed issue counts
	IssueCountHistory          []int // Daily total issue counts
}

// DisplayName returns the cycle name or a generated one based on number.
func (c Cycle) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Cycle #%d", c.Number)
}

// ProgressPercent returns progress as an integer percentage (0–100).
func (c Cycle) ProgressPercent() int {
	return int(c.Progress * 100)
}

// CycleUpdateInput is a custom scalar type for Linear's CycleUpdateInput.
type CycleUpdateInput map[string]interface{}

// GetGraphQLType returns the GraphQL type name.
func (CycleUpdateInput) GetGraphQLType() string {
	return "CycleUpdateInput"
}

// MarshalJSON implements json.Marshaler for CycleUpdateInput.
func (cu CycleUpdateInput) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(cu))
}

// parseIntSlice converts a []float64 (from JSON numbers) to []int.
func parseIntSlice(f []float64) []int {
	result := make([]int, len(f))
	for i, v := range f {
		result[i] = int(v)
	}
	return result
}

// ListCycles fetches all cycles for a team ordered by createdAt.
// Uses a raw HTTP request to decode JSON array fields (completedIssueCountHistory,
// issueCountHistory) that the shurcooL/graphql library cannot decode natively.
func (c *Client) ListCycles(ctx context.Context, teamID string) ([]Cycle, error) {
	type rawCycleNode struct {
		ID                         string    `json:"id"`
		Name                       *string   `json:"name"`
		Number                     float64   `json:"number"`
		StartsAt                   *string   `json:"startsAt"`
		EndsAt                     *string   `json:"endsAt"`
		Progress                   *float64  `json:"progress"`
		CompletedIssueCountHistory []float64 `json:"completedIssueCountHistory"`
		IssueCountHistory          []float64 `json:"issueCountHistory"`
	}
	type rawResponse struct {
		Data struct {
			Team struct {
				Cycles struct {
					Nodes []rawCycleNode `json:"nodes"`
				} `json:"cycles"`
			} `json:"team"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	gqlQuery := `query ListCycles($teamId: String!) {
		team(id: $teamId) {
			cycles(orderBy: createdAt) {
				nodes {
					id
					name
					number
					startsAt
					endsAt
					progress
					completedIssueCountHistory
					issueCountHistory
				}
			}
		}
	}`

	reqBody, err := json.Marshal(map[string]interface{}{
		"query":     gqlQuery,
		"variables": map[string]interface{}{"teamId": teamID},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cycles query: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build cycles request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// Authorization header is added by authTransport on c.httpClient

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logger.ErrorWithErr(err, "linearapi.cycles: ListCycles HTTP failed team_id=%s", teamID)
		return nil, fmt.Errorf("list cycles for team %s: %w", teamID, err)
	}
	defer resp.Body.Close()

	var raw rawResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		logger.ErrorWithErr(err, "linearapi.cycles: ListCycles decode failed team_id=%s", teamID)
		return nil, fmt.Errorf("decode cycles response for team %s: %w", teamID, err)
	}
	if len(raw.Errors) > 0 {
		return nil, fmt.Errorf("list cycles for team %s: %s", teamID, raw.Errors[0].Message)
	}

	nodes := raw.Data.Team.Cycles.Nodes
	cycles := make([]Cycle, 0, len(nodes))
	for _, node := range nodes {
		cyc := Cycle{
			ID:                         node.ID,
			Number:                     int(node.Number),
			TeamID:                     teamID,
			CompletedIssueCountHistory: parseIntSlice(node.CompletedIssueCountHistory),
			IssueCountHistory:          parseIntSlice(node.IssueCountHistory),
		}
		if node.Name != nil {
			cyc.Name = *node.Name
		}
		if node.StartsAt != nil {
			cyc.StartsAt = parseTime(*node.StartsAt)
		}
		if node.EndsAt != nil {
			cyc.EndsAt = parseTime(*node.EndsAt)
		}
		if node.Progress != nil {
			cyc.Progress = *node.Progress
		}
		cycles = append(cycles, cyc)
	}
	return cycles, nil
}

// StartCycle sets a cycle's startsAt to now (using the cycleUpdate mutation).
func (c *Client) StartCycle(ctx context.Context, cycleID string) error {
	var mutation struct {
		CycleUpdate struct {
			Success graphql.Boolean
		} `graphql:"cycleUpdate(id: $id, input: $input)"`
	}

	now := time.Now().UTC().Format(time.RFC3339)
	input := CycleUpdateInput{"startsAt": now}
	variables := map[string]interface{}{
		"id":    graphql.String(cycleID),
		"input": input,
	}

	if err := c.client.Mutate(ctx, &mutation, variables); err != nil {
		logger.ErrorWithErr(err, "linearapi.cycles: StartCycle failed cycle_id=%s", cycleID)
		return fmt.Errorf("start cycle %s: %w", cycleID, err)
	}

	if !bool(mutation.CycleUpdate.Success) {
		return fmt.Errorf("start cycle %s: operation failed", cycleID)
	}
	return nil
}

// AddIssueToCycle assigns an issue to a cycle using issueUpdate.
func (c *Client) AddIssueToCycle(ctx context.Context, issueID, cycleID string) error {
	var mutation struct {
		IssueUpdate struct {
			Success graphql.Boolean
		} `graphql:"issueUpdate(id: $id, input: $input)"`
	}

	input := IssueUpdateInput{"cycleId": cycleID}
	variables := map[string]interface{}{
		"id":    graphql.String(issueID),
		"input": input,
	}

	if err := c.client.Mutate(ctx, &mutation, variables); err != nil {
		logger.ErrorWithErr(err, "linearapi.cycles: AddIssueToCycle failed issue_id=%s cycle_id=%s", issueID, cycleID)
		return fmt.Errorf("add issue %s to cycle %s: %w", issueID, cycleID, err)
	}

	if !bool(mutation.IssueUpdate.Success) {
		return fmt.Errorf("add issue %s to cycle %s: operation failed", issueID, cycleID)
	}
	return nil
}

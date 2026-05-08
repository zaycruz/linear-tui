package linearapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/roeyazroel/linear-tui/internal/logger"
	"github.com/shurcooL/graphql"
)

// Cycle represents a Linear cycle (sprint).
type Cycle struct {
	ID       string
	Name     string
	Number   int
	StartsAt time.Time
	EndsAt   time.Time
	Progress float64 // 0.0–1.0
	TeamID   string
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

// ListCycles fetches all cycles for a team ordered by createdAt.
func (c *Client) ListCycles(ctx context.Context, teamID string) ([]Cycle, error) {
	var query struct {
		Team struct {
			Cycles struct {
				Nodes []struct {
					ID       graphql.String
					Name     *graphql.String
					Number   graphql.Float
					StartsAt *graphql.String
					EndsAt   *graphql.String
					Progress *graphql.Float
				}
			} `graphql:"cycles(orderBy: createdAt)"`
		} `graphql:"team(id: $teamId)"`
	}

	variables := map[string]interface{}{
		"teamId": graphql.String(teamID),
	}

	if err := c.client.Query(ctx, &query, variables); err != nil {
		logger.ErrorWithErr(err, "linearapi.cycles: ListCycles failed team_id=%s", teamID)
		return nil, fmt.Errorf("list cycles for team %s: %w", teamID, err)
	}

	cycles := make([]Cycle, 0, len(query.Team.Cycles.Nodes))
	for _, node := range query.Team.Cycles.Nodes {
		cyc := Cycle{
			ID:     string(node.ID),
			Number: int(node.Number),
			TeamID: teamID,
		}
		if node.Name != nil {
			cyc.Name = string(*node.Name)
		}
		if node.StartsAt != nil {
			cyc.StartsAt = parseTime(string(*node.StartsAt))
		}
		if node.EndsAt != nil {
			cyc.EndsAt = parseTime(string(*node.EndsAt))
		}
		if node.Progress != nil {
			cyc.Progress = float64(*node.Progress)
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

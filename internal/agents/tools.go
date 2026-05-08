package agents

import "encoding/json"

// ToolExecutor is implemented by the TUI app to handle both Linear API calls and UI navigation.
type ToolExecutor interface {
	// Linear mutations
	ExecUpdateIssue(issueID string, fields map[string]interface{}) (string, error)
	ExecCreateIssue(teamID, title, description, projectID, stateID, assigneeID string, priority int) (string, error)
	ExecAddComment(issueID, body string) error
	ExecAddToCycle(cycleID string, issueIDs []string) error
	ExecStartCycle(cycleID string) error
	ExecArchiveIssue(issueID string) error

	// TUI navigation (fire-and-forget, return immediately)
	NavToProject(nameOrID string)
	NavToCycle(nameOrID string)
	NavToAllIssues()
	NavFilterMyIssues(enabled bool)
	NavOpenOverlay(name string) // "velocity", "stats", "triage", "fuzzy_finder"
	NavRefresh()

	// Data reads (for agent reasoning)
	ReadCurrentIssues() []map[string]interface{}
	ReadCycles() []map[string]interface{}
	ReadWorkflowStates() []map[string]interface{}
	ReadProjects() []map[string]interface{}
	ReadUsers() []map[string]interface{}
	ReadCurrentTeamID() string
	ReadCurrentUserID() string
}

// toolDefs is the full set of tools exposed to the agent.
var toolDefs = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "update_issue",
			"description": "Update one or more fields on a Linear issue. Use issue identifier (e.g. RAA-123) or ID.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issue_id":   prop("string", "Issue identifier or ID (e.g. RAA-123)"),
					"title":      prop("string", "New title"),
					"status":     prop("string", "Workflow state name (e.g. 'In Progress', 'Done')"),
					"priority":   prop("integer", "Priority: 0=no priority, 1=urgent, 2=high, 3=normal, 4=low"),
					"assignee":   prop("string", "Assignee name or user ID. Use 'me' to assign to current user."),
					"estimate":   prop("number", "Story point estimate"),
					"due_date":   prop("string", "Due date as YYYY-MM-DD, or empty string to clear"),
				},
				"required": []string{"issue_id"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "create_issue",
			"description": "Create a new Linear issue",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title":       prop("string", "Issue title"),
					"description": prop("string", "Issue description (markdown)"),
					"project":     prop("string", "Project name or ID to assign the issue to"),
					"status":      prop("string", "Initial workflow state name"),
					"assignee":    prop("string", "Assignee name, user ID, or 'me'"),
					"priority":    prop("integer", "Priority: 0=no priority, 1=urgent, 2=high, 3=normal, 4=low"),
				},
				"required": []string{"title"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "add_comment",
			"description": "Add a comment to a Linear issue",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issue_id": prop("string", "Issue identifier or ID"),
					"body":     prop("string", "Comment body (markdown supported)"),
				},
				"required": []string{"issue_id", "body"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "add_to_cycle",
			"description": "Add one or more issues to a cycle (sprint)",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cycle":     prop("string", "Cycle name or ID"),
					"issue_ids": arrayProp("string", "List of issue identifiers or IDs"),
				},
				"required": []string{"cycle", "issue_ids"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "start_cycle",
			"description": "Start a cycle (sprint)",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cycle": prop("string", "Cycle name or ID to start"),
				},
				"required": []string{"cycle"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "archive_issue",
			"description": "Archive a Linear issue",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"issue_id": prop("string", "Issue identifier or ID"),
				},
				"required": []string{"issue_id"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "navigate",
			"description": "Navigate the TUI to a specific view: a project, cycle, or 'all issues'",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": prop("string", "What to navigate to: project name, cycle name, 'all issues', 'my issues'"),
				},
				"required": []string{"target"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "show_overlay",
			"description": "Open a TUI overlay panel for metrics or actions",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"panel": prop("string", "One of: velocity, stats, triage, fuzzy_finder, kanban"),
				},
				"required": []string{"panel"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_issues",
			"description": "Return the current issue list for agent reasoning. Call this to get issue IDs before updating them.",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_cycles",
			"description": "Return all cycles with their names and IDs",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_workflow_states",
			"description": "Return all workflow states (statuses) for the current team",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_projects",
			"description": "Return all projects for the current team",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "list_users",
			"description": "Return all team members with their names and IDs",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "refresh",
			"description": "Refresh the current issue list from Linear",
			"parameters": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
	},
}

func prop(typ, desc string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": desc}
}

func arrayProp(itemType, desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": desc,
		"items":       map[string]interface{}{"type": itemType},
	}
}

// ToolCall represents a function call from the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Args parses the tool call arguments into a map.
func (tc ToolCall) Args() map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal([]byte(tc.Function.Arguments), &m)
	return m
}

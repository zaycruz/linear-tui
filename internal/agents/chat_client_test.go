package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mockExecutor records calls for assertions.
type mockExecutor struct {
	updatedIssue    string
	updatedFields   map[string]interface{}
	createdTitle    string
	commentIssueID  string
	commentBody     string
	cycleAdded      string
	cycleIssues     []string
	cycleStarted    string
	archivedIssue   string
	navProject      string
	navCycle        string
	navAllIssues    bool
	navMyIssues     bool
	navOverlay      string
	refreshed       bool
	teamID          string
	userID          string
}

func (m *mockExecutor) ExecUpdateIssue(id string, fields map[string]interface{}) (string, error) {
	m.updatedIssue = id
	m.updatedFields = fields
	return fmt.Sprintf("Updated %s", id), nil
}
func (m *mockExecutor) ExecCreateIssue(teamID, title, desc, project, status, assignee string, priority int) (string, error) {
	m.createdTitle = title
	return "RAA-42", nil
}
func (m *mockExecutor) ExecAddComment(issueID, body string) error {
	m.commentIssueID = issueID
	m.commentBody = body
	return nil
}
func (m *mockExecutor) ExecAddToCycle(cycle string, issues []string) error {
	m.cycleAdded = cycle
	m.cycleIssues = issues
	return nil
}
func (m *mockExecutor) ExecStartCycle(cycle string) error {
	m.cycleStarted = cycle
	return nil
}
func (m *mockExecutor) ExecArchiveIssue(id string) error {
	m.archivedIssue = id
	return nil
}
func (m *mockExecutor) NavToProject(n string)            { m.navProject = n }
func (m *mockExecutor) NavToCycle(n string)              { m.navCycle = n }
func (m *mockExecutor) NavToAllIssues()                  { m.navAllIssues = true }
func (m *mockExecutor) NavFilterMyIssues(enabled bool)   { m.navMyIssues = enabled }
func (m *mockExecutor) NavOpenOverlay(panel string)      { m.navOverlay = panel }
func (m *mockExecutor) NavRefresh()                      { m.refreshed = true }
func (m *mockExecutor) ReadCurrentIssues() []map[string]interface{} {
	return []map[string]interface{}{{"id": "abc", "identifier": "RAA-1", "title": "Test issue"}}
}
func (m *mockExecutor) ReadCycles() []map[string]interface{} {
	return []map[string]interface{}{{"id": "c1", "name": "Cycle #1", "progress": 50.0}}
}
func (m *mockExecutor) ReadWorkflowStates() []map[string]interface{} {
	return []map[string]interface{}{{"id": "s1", "name": "In Progress", "type": "started"}}
}
func (m *mockExecutor) ReadProjects() []map[string]interface{} {
	return []map[string]interface{}{{"id": "p1", "name": "Cargo3001 Platform"}}
}
func (m *mockExecutor) ReadUsers() []map[string]interface{} {
	return []map[string]interface{}{{"id": "u1", "name": "Zay", "me": true}}
}
func (m *mockExecutor) ReadCurrentTeamID() string { return m.teamID }
func (m *mockExecutor) ReadCurrentUserID() string { return m.userID }

func makeTC(name string, args map[string]interface{}) ToolCall {
	raw, _ := json.Marshal(args)
	tc := ToolCall{ID: "tc1", Type: "function"}
	tc.Function.Name = name
	tc.Function.Arguments = string(raw)
	return tc
}

func TestExecuteToolCall_UpdateIssue(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("update_issue", map[string]interface{}{"issue_id": "RAA-5", "status": "Done"})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.updatedIssue != "RAA-5" {
		t.Errorf("expected RAA-5, got %s", ex.updatedIssue)
	}
	if !strings.Contains(result, "RAA-5") {
		t.Errorf("expected result to mention RAA-5, got %s", result)
	}
}

func TestExecuteToolCall_CreateIssue(t *testing.T) {
	ex := &mockExecutor{teamID: "team1"}
	tc := makeTC("create_issue", map[string]interface{}{"title": "New bug", "priority": float64(2)})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.createdTitle != "New bug" {
		t.Errorf("expected 'New bug', got %s", ex.createdTitle)
	}
	if !strings.Contains(result, "RAA-42") {
		t.Errorf("expected result to contain RAA-42, got %s", result)
	}
}

func TestExecuteToolCall_AddComment(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("add_comment", map[string]interface{}{"issue_id": "RAA-3", "body": "looks good"})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.commentIssueID != "RAA-3" || ex.commentBody != "looks good" {
		t.Errorf("comment not recorded correctly")
	}
	if result != "Comment added" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExecuteToolCall_AddToCycle(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("add_to_cycle", map[string]interface{}{
		"cycle":     "Cycle #1",
		"issue_ids": []interface{}{"RAA-1", "RAA-2"},
	})
	_, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.cycleAdded != "Cycle #1" {
		t.Errorf("expected Cycle #1, got %s", ex.cycleAdded)
	}
	if len(ex.cycleIssues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(ex.cycleIssues))
	}
}

func TestExecuteToolCall_StartCycle(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("start_cycle", map[string]interface{}{"cycle": "Sprint 3"})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.cycleStarted != "Sprint 3" {
		t.Errorf("expected Sprint 3, got %s", ex.cycleStarted)
	}
	if result != "Cycle started" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExecuteToolCall_ArchiveIssue(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("archive_issue", map[string]interface{}{"issue_id": "RAA-7"})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.archivedIssue != "RAA-7" {
		t.Errorf("expected RAA-7, got %s", ex.archivedIssue)
	}
	if result != "Issue archived" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExecuteToolCall_Navigate_AllIssues(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("navigate", map[string]interface{}{"target": "all issues"})
	_, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.navAllIssues {
		t.Error("expected NavToAllIssues to be called")
	}
}

func TestExecuteToolCall_Navigate_MyIssues(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("navigate", map[string]interface{}{"target": "my issues"})
	_, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.navMyIssues {
		t.Error("expected NavFilterMyIssues(true) to be called")
	}
}

func TestExecuteToolCall_Navigate_Project(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("navigate", map[string]interface{}{"target": "cargo3001"})
	_, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if ex.navProject != "cargo3001" {
		t.Errorf("expected navProject=cargo3001, got %q", ex.navProject)
	}
	if ex.navCycle != "cargo3001" {
		t.Errorf("expected navCycle=cargo3001 (tried both), got %q", ex.navCycle)
	}
}

func TestExecuteToolCall_ShowOverlay(t *testing.T) {
	for _, panel := range []string{"velocity", "stats", "triage", "fuzzy_finder"} {
		ex := &mockExecutor{}
		tc := makeTC("show_overlay", map[string]interface{}{"panel": panel})
		result, err := executeToolCall(context.Background(), tc, ex)
		if err != nil {
			t.Fatalf("panel %s: %v", panel, err)
		}
		if ex.navOverlay != panel {
			t.Errorf("panel %s: expected navOverlay=%s, got %q", panel, panel, ex.navOverlay)
		}
		if !strings.Contains(result, panel) {
			t.Errorf("panel %s: result should mention panel name, got %q", panel, result)
		}
	}
}

func TestExecuteToolCall_ListTools(t *testing.T) {
	ex := &mockExecutor{}
	for _, name := range []string{"list_issues", "list_cycles", "list_workflow_states", "list_projects", "list_users"} {
		tc := makeTC(name, map[string]interface{}{})
		result, err := executeToolCall(context.Background(), tc, ex)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result == "" {
			t.Errorf("%s: empty result", name)
		}
		// Result should be valid JSON array
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(result), &arr); err != nil {
			t.Errorf("%s: result not valid JSON array: %s", name, result)
		}
		if len(arr) == 0 {
			t.Errorf("%s: expected non-empty array", name)
		}
	}
}

func TestExecuteToolCall_Refresh(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("refresh", map[string]interface{}{})
	result, err := executeToolCall(context.Background(), tc, ex)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.refreshed {
		t.Error("expected NavRefresh to be called")
	}
	if result != "Refreshed" {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExecuteToolCall_UnknownTool(t *testing.T) {
	ex := &mockExecutor{}
	tc := makeTC("nonexistent_tool", map[string]interface{}{})
	_, err := executeToolCall(context.Background(), tc, ex)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected 'unknown tool' error, got: %v", err)
	}
}

func TestSummarizeArgs(t *testing.T) {
	args := map[string]interface{}{"issue_id": "RAA-5", "status": "Done"}
	result := summarizeArgs(args)
	if result == "" {
		t.Error("expected non-empty summarizeArgs result")
	}
}

func TestToStringSlice(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	result := toStringSlice(input)
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
	for i, v := range []string{"a", "b", "c"} {
		if result[i] != v {
			t.Errorf("index %d: expected %s, got %s", i, v, result[i])
		}
	}

	if toStringSlice(nil) != nil {
		t.Error("expected nil for nil input")
	}
}

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/roeyazroel/linear-tui/internal/agents"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// AppExecutor implements agents.ToolExecutor backed by the running App.
type AppExecutor struct {
	app *App
}

// ----- Linear mutations -----

func (e *AppExecutor) ExecUpdateIssue(issueIDOrIdentifier string, fields map[string]interface{}) (string, error) {
	ctx := context.Background()

	// Resolve issue ID
	issueID, err := e.resolveIssueID(ctx, issueIDOrIdentifier)
	if err != nil {
		return "", err
	}

	input := linearapi.UpdateIssueInput{ID: issueID}
	changed := []string{}

	if v, ok := fields["title"].(string); ok && v != "" {
		input.Title = &v
		changed = append(changed, "title")
	}
	if v, ok := fields["status"].(string); ok && v != "" {
		stateID, err := e.resolveStateID(v)
		if err != nil {
			return "", err
		}
		input.StateID = &stateID
		changed = append(changed, "status→"+v)
	}
	if v, ok := fields["priority"].(float64); ok {
		p := int(v)
		input.Priority = &p
		changed = append(changed, fmt.Sprintf("priority→%d", p))
	}
	if v, ok := fields["assignee"].(string); ok && v != "" {
		userID, err := e.resolveUserID(v)
		if err != nil {
			return "", err
		}
		input.AssigneeID = &userID
		changed = append(changed, "assignee→"+v)
	}
	if v, ok := fields["estimate"].(float64); ok {
		input.Estimate = &v
		changed = append(changed, fmt.Sprintf("estimate→%.0f", v))
	}
	if v, ok := fields["due_date"].(string); ok {
		input.DueDate = &v
		changed = append(changed, "due_date→"+v)
	}

	if _, err := e.app.api.UpdateIssue(ctx, input); err != nil {
		return "", err
	}

	e.app.queueUpdateDraw(func() {
		e.app.refreshIssues()
	})

	return fmt.Sprintf("Updated %s: %s", issueIDOrIdentifier, strings.Join(changed, ", ")), nil
}

func (e *AppExecutor) ExecCreateIssue(teamID, title, description, project, status, assignee string, priority int) (string, error) {
	ctx := context.Background()

	if teamID == "" {
		teamID = e.ReadCurrentTeamID()
	}
	if teamID == "" {
		return "", fmt.Errorf("no team selected — navigate to a team first")
	}

	input := linearapi.CreateIssueInput{
		TeamID:      teamID,
		Title:       title,
		Description: description,
		Priority:    priority,
	}

	if project != "" {
		pid, err := e.resolveProjectID(project)
		if err == nil {
			input.ProjectID = pid
		}
	}
	if status != "" {
		sid, err := e.resolveStateID(status)
		if err == nil {
			input.StateID = sid
		}
	}
	if assignee != "" {
		uid, _ := e.resolveUserID(assignee)
		input.AssigneeID = uid
	}

	issue, err := e.app.api.CreateIssue(ctx, input)
	if err != nil {
		return "", err
	}

	e.app.queueUpdateDraw(func() {
		e.app.refreshIssues()
	})

	return issue.Identifier, nil
}

func (e *AppExecutor) ExecAddComment(issueIDOrIdentifier, body string) error {
	ctx := context.Background()
	issueID, err := e.resolveIssueID(ctx, issueIDOrIdentifier)
	if err != nil {
		return err
	}
	_, err = e.app.api.CreateComment(ctx, linearapi.CreateCommentInput{
		IssueID: issueID,
		Body:    body,
	})
	return err
}

func (e *AppExecutor) ExecAddToCycle(cycleNameOrID string, issueIDs []string) error {
	ctx := context.Background()
	cycleID, err := e.resolveCycleID(cycleNameOrID)
	if err != nil {
		return err
	}
	for _, iid := range issueIDs {
		resolved, err := e.resolveIssueID(ctx, iid)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", iid, err)
		}
		if err := e.app.api.AddIssueToCycle(ctx, resolved, cycleID); err != nil {
			return err
		}
	}
	return nil
}

func (e *AppExecutor) ExecStartCycle(cycleNameOrID string) error {
	cycleID, err := e.resolveCycleID(cycleNameOrID)
	if err != nil {
		return err
	}
	return e.app.api.StartCycle(context.Background(), cycleID)
}

func (e *AppExecutor) ExecArchiveIssue(issueIDOrIdentifier string) error {
	ctx := context.Background()
	issueID, err := e.resolveIssueID(ctx, issueIDOrIdentifier)
	if err != nil {
		return err
	}
	err = e.app.api.ArchiveIssue(ctx, issueID)
	if err == nil {
		e.app.queueUpdateDraw(func() {
			e.app.refreshIssues()
		})
	}
	return err
}

// ----- TUI navigation (fire-and-forget) -----

func (e *AppExecutor) NavToProject(nameOrID string) {
	e.app.queueUpdateDraw(func() {
		lower := strings.ToLower(nameOrID)
		var best *linearapi.Project
		for i := range e.app.teamProjects {
			p := &e.app.teamProjects[i]
			pLower := strings.ToLower(p.Name)
			if strings.EqualFold(p.Name, nameOrID) || p.ID == nameOrID {
				best = p
				break
			}
			if best == nil && strings.Contains(pLower, lower) {
				best = p
			}
		}
		if best != nil {
			node := &NavigationNode{
				ID:        best.ID,
				Text:      best.Name,
				TeamID:    best.TeamID,
				IsProject: true,
			}
			e.app.onNavigationSelected(node)
		}
	})
}

func (e *AppExecutor) NavToCycle(nameOrID string) {
	e.app.queueUpdateDraw(func() {
		lower := strings.ToLower(nameOrID)
		for _, c := range e.app.cycles {
			dn := c.DisplayName()
			if strings.EqualFold(dn, nameOrID) || c.ID == nameOrID ||
				strings.Contains(strings.ToLower(dn), lower) {
				node := &NavigationNode{
					ID:      c.ID,
					Text:    dn,
					TeamID:  c.TeamID,
					IsCycle: true,
					CycleID: c.ID,
				}
				e.app.onNavigationSelected(node)
				return
			}
		}
	})
}

func (e *AppExecutor) NavToAllIssues() {
	e.app.queueUpdateDraw(func() {
		if e.app.selectedNavigation != nil {
			node := &NavigationNode{
				ID:     e.app.selectedNavigation.TeamID,
				Text:   "All Issues",
				TeamID: e.app.selectedNavigation.TeamID,
				IsTeam: true,
			}
			e.app.onNavigationSelected(node)
		}
	})
}

func (e *AppExecutor) NavFilterMyIssues(enabled bool) {
	e.app.queueUpdateDraw(func() {
		if e.app.filterAssigneeMe != enabled {
			e.app.toggleFilterAssigneeMe()
		}
	})
}

func (e *AppExecutor) NavOpenOverlay(panel string) {
	e.app.queueUpdateDraw(func() {
		switch panel {
		case "velocity":
			if e.app.velocityModal != nil {
				teamName := ""
				if e.app.selectedNavigation != nil {
					teamName = e.app.selectedNavigation.Text
				}
				e.app.issuesMu.RLock()
				cycles := e.app.cycles
				e.app.issuesMu.RUnlock()
				e.app.velocityModal.Show(cycles, teamName)
			}
		case "stats":
			if e.app.statsModal != nil {
				teamName := ""
				if e.app.selectedNavigation != nil {
					teamName = e.app.selectedNavigation.Text
				}
				e.app.issuesMu.RLock()
				issues := make([]linearapi.Issue, len(e.app.issues))
				copy(issues, e.app.issues)
				e.app.issuesMu.RUnlock()
				e.app.statsModal.Show(issues, teamName)
			}
		case "triage":
			if e.app.triageModal != nil {
				e.app.issuesMu.RLock()
				issues := make([]linearapi.Issue, len(e.app.issues))
				copy(issues, e.app.issues)
				e.app.issuesMu.RUnlock()
				e.app.triageModal.Start(issues)
			}
		case "fuzzy_finder":
			if e.app.fuzzyFinder != nil {
				e.app.fuzzyFinder.Show()
			}
		}
	})
}

func (e *AppExecutor) NavRefresh() {
	e.app.queueUpdateDraw(func() {
		go e.app.refreshIssues()
	})
}

// ----- Data reads -----

func (e *AppExecutor) ReadCurrentIssues() []map[string]interface{} {
	e.app.issuesMu.RLock()
	issues := make([]linearapi.Issue, len(e.app.issues))
	copy(issues, e.app.issues)
	e.app.issuesMu.RUnlock()

	out := make([]map[string]interface{}, 0, len(issues))
	for _, iss := range issues {
		out = append(out, map[string]interface{}{
			"id":         iss.ID,
			"identifier": iss.Identifier,
			"title":      iss.Title,
			"status":     iss.State,
			"priority":   iss.Priority,
			"assignee":   iss.Assignee,
			"due_date":   iss.DueDate,
		})
	}
	return out
}

func (e *AppExecutor) ReadCycles() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(e.app.cycles))
	for _, c := range e.app.cycles {
		out = append(out, map[string]interface{}{
			"id":       c.ID,
			"name":     c.DisplayName(),
			"progress": c.ProgressPercent(),
		})
	}
	return out
}

func (e *AppExecutor) ReadWorkflowStates() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(e.app.workflowStates))
	for _, s := range e.app.workflowStates {
		out = append(out, map[string]interface{}{
			"id":   s.ID,
			"name": s.Name,
			"type": s.Type,
		})
	}
	return out
}

func (e *AppExecutor) ReadProjects() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(e.app.teamProjects))
	for _, p := range e.app.teamProjects {
		out = append(out, map[string]interface{}{
			"id":   p.ID,
			"name": p.Name,
		})
	}
	return out
}

func (e *AppExecutor) ReadUsers() []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(e.app.teamUsers))
	for _, u := range e.app.teamUsers {
		out = append(out, map[string]interface{}{
			"id":   u.ID,
			"name": u.Name,
			"me":   u.IsMe,
		})
	}
	return out
}

func (e *AppExecutor) ReadCurrentTeamID() string {
	if e.app.selectedNavigation != nil {
		return e.app.selectedNavigation.TeamID
	}
	return ""
}

func (e *AppExecutor) ReadCurrentUserID() string {
	if e.app.currentUser != nil {
		return e.app.currentUser.ID
	}
	return ""
}

// ----- Resolution helpers -----

func (e *AppExecutor) resolveIssueID(ctx context.Context, identifier string) (string, error) {
	// If it looks like a UUID, use directly
	if len(identifier) > 30 && !strings.Contains(identifier, "-") {
		return identifier, nil
	}
	// Search in current issues first
	e.app.issuesMu.RLock()
	for _, iss := range e.app.issues {
		if strings.EqualFold(iss.Identifier, identifier) || iss.ID == identifier {
			id := iss.ID
			e.app.issuesMu.RUnlock()
			return id, nil
		}
	}
	e.app.issuesMu.RUnlock()
	// Fall back to API fetch
	issue, err := e.app.fetchIssueByID(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("issue %q not found", identifier)
	}
	return issue.ID, nil
}

func (e *AppExecutor) resolveStateID(name string) (string, error) {
	for _, s := range e.app.workflowStates {
		if strings.EqualFold(s.Name, name) || s.ID == name {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("workflow state %q not found", name)
}

func (e *AppExecutor) resolveUserID(nameOrID string) (string, error) {
	if strings.EqualFold(nameOrID, "me") {
		if e.app.currentUser != nil {
			return e.app.currentUser.ID, nil
		}
		return "", fmt.Errorf("current user unknown")
	}
	for _, u := range e.app.teamUsers {
		if strings.EqualFold(u.Name, nameOrID) || u.ID == nameOrID {
			return u.ID, nil
		}
	}
	return "", fmt.Errorf("user %q not found", nameOrID)
}

func (e *AppExecutor) resolveCycleID(nameOrID string) (string, error) {
	for _, c := range e.app.cycles {
		if strings.EqualFold(c.DisplayName(), nameOrID) || c.ID == nameOrID {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("cycle %q not found", nameOrID)
}

func (e *AppExecutor) resolveProjectID(nameOrID string) (string, error) {
	for _, p := range e.app.teamProjects {
		if strings.EqualFold(p.Name, nameOrID) || p.ID == nameOrID {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("project %q not found", nameOrID)
}

// Ensure AppExecutor satisfies the interface at compile time.
var _ agents.ToolExecutor = (*AppExecutor)(nil)

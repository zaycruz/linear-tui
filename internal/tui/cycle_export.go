package tui

import (
	"fmt"
	"strings"

	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// buildCycleReport generates a markdown report for a cycle.
func buildCycleReport(cyc *linearapi.Cycle, issues []linearapi.Issue, projects []linearapi.Project) string {
	if cyc == nil {
		return ""
	}

	// Build project name lookup
	projectNames := make(map[string]string)
	for _, p := range projects {
		projectNames[p.ID] = p.Name
	}

	// Date range header
	dateRange := ""
	if !cyc.StartsAt.IsZero() && !cyc.EndsAt.IsZero() {
		dateRange = fmt.Sprintf("— %s–%s, %d", cyc.StartsAt.Format("Jan 2"), cyc.EndsAt.Format("Jan 2"), cyc.EndsAt.Year())
	}

	// Compute progress from last history entry
	done := 0
	total := len(issues)
	if n := len(cyc.CompletedIssueCountHistory); n > 0 {
		done = cyc.CompletedIssueCountHistory[n-1]
	}
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}

	// Group issues by project
	byProject := make(map[string][]linearapi.Issue)
	noProject := "No Project"
	for _, issue := range issues {
		key := noProject
		if issue.ProjectID != "" {
			if name, ok := projectNames[issue.ProjectID]; ok {
				key = name
			} else {
				key = issue.ProjectID
			}
		}
		byProject[key] = append(byProject[key], issue)
	}

	// Count states
	inProgress := 0
	backlog := 0
	for _, issue := range issues {
		stateLower := strings.ToLower(issue.State)
		if strings.Contains(stateLower, "progress") || strings.Contains(stateLower, "review") ||
			strings.Contains(stateLower, "started") {
			inProgress++
		} else if !strings.Contains(stateLower, "done") && !strings.Contains(stateLower, "complet") &&
			!strings.Contains(stateLower, "cancel") {
			backlog++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s %s\n\n", cyc.DisplayName(), dateRange))
	sb.WriteString(fmt.Sprintf("**Progress:** %d/%d done (%d%%)\n\n", done, total, pct))
	sb.WriteString("## By Project\n\n")

	// Write each project section
	for projectName, projectIssues := range byProject {
		sb.WriteString(fmt.Sprintf("### %s\n", projectName))
		for _, issue := range projectIssues {
			icon := "🔄"
			stateLower := strings.ToLower(issue.State)
			if strings.Contains(stateLower, "done") || strings.Contains(stateLower, "complet") {
				icon = "✅"
			} else if strings.Contains(stateLower, "cancel") {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("- %s %s %s (%s)\n", icon, issue.Identifier, issue.Title, issue.State))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Summary\n")
	sb.WriteString(fmt.Sprintf("Total: %d issues | Done: %d | In Progress: %d | Backlog: %d\n",
		total, done, inProgress, backlog))

	return sb.String()
}

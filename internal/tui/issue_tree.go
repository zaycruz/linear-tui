package tui

import (
	"sort"

	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// BuildIssueRowsGroupedByProject builds rows like BuildIssueRows but groups them by
// project, inserting a non-selectable header row at the start of each group.
// projectNames maps projectID → display name; empty string projectID → "No Project".
func BuildIssueRowsGroupedByProject(issues []linearapi.Issue, expanded map[string]bool, projectNames map[string]string) ([]IssueRow, map[string]*linearapi.Issue) {
	idToIssue := make(map[string]*linearapi.Issue, len(issues))
	for i := range issues {
		idToIssue[issues[i].ID] = &issues[i]
	}

	// Group issues by ProjectID
	type projectGroup struct {
		projectID string
		issues    []*linearapi.Issue
	}
	groupMap := make(map[string]*projectGroup)
	groupOrder := make([]string, 0)

	for i := range issues {
		issue := &issues[i]
		pid := issue.ProjectID
		if _, exists := groupMap[pid]; !exists {
			groupMap[pid] = &projectGroup{projectID: pid}
			groupOrder = append(groupOrder, pid)
		}
		groupMap[pid].issues = append(groupMap[pid].issues, issue)
	}

	// Sort groups: non-empty project names first (alphabetical), then "No Project"
	sort.Slice(groupOrder, func(i, j int) bool {
		a, b := groupOrder[i], groupOrder[j]
		nameA := projectNameFor(a, projectNames)
		nameB := projectNameFor(b, projectNames)
		if nameA == "" && nameB != "" {
			return false
		}
		if nameA != "" && nameB == "" {
			return true
		}
		return nameA < nameB
	})

	var rows []IssueRow

	for _, pid := range groupOrder {
		grp := groupMap[pid]
		if len(grp.issues) == 0 {
			continue
		}

		// Insert project header row
		pName := projectNameFor(pid, projectNames)
		rows = append(rows, IssueRow{
			IsProjectHeader: true,
			ProjectName:     pName,
		})

		// Build per-group rows (no hierarchy within groups for simplicity)
		for _, issue := range grp.issues {
			hasChildren := len(issue.Children) > 0
			isExpanded := expanded[issue.ID]
			rows = append(rows, IssueRow{
				IssueID:     issue.ID,
				Level:       0,
				IsParent:    hasChildren,
				HasChildren: hasChildren,
				IsExpanded:  isExpanded,
			})
		}
	}

	return rows, idToIssue
}

// projectNameFor returns a display name for a project ID using the provided name map.
func projectNameFor(projectID string, names map[string]string) string {
	if projectID == "" {
		return "No Project"
	}
	if name, ok := names[projectID]; ok && name != "" {
		return name
	}
	return "No Project"
}

// IssueRow represents a single row in the issues table with hierarchy info.
type IssueRow struct {
	IssueID     string // Reference to the issue
	Level       int    // Nesting level (0 = top-level, 1 = child, etc.)
	IsParent    bool   // True if this issue has children
	HasChildren bool   // True if this issue has children (same as IsParent for now)
	IsExpanded  bool   // True if children are shown (only meaningful when HasChildren is true)

	// Project group header support (cycle view)
	IsProjectHeader bool   // True if this is a non-selectable project header row
	ProjectName     string // Project name for header rows
}

// BuildIssueRows constructs a flattened list of rows for table rendering.
// It builds a hierarchical view where parent issues can be expanded/collapsed.
// Returns the rows and a map for quick issue lookup by ID.
func BuildIssueRows(issues []linearapi.Issue, expanded map[string]bool) ([]IssueRow, map[string]*linearapi.Issue) {
	idToIssue := make(map[string]*linearapi.Issue, len(issues))
	for i := range issues {
		idToIssue[issues[i].ID] = &issues[i]
	}

	// Separate parent issues (no parent in our list) from children
	// An issue is a "top-level" issue if:
	// 1. It has no parent (issue.Parent == nil), OR
	// 2. Its parent is not in our fetched list (orphan sub-issue)
	var topLevel []*linearapi.Issue
	childrenByParent := make(map[string][]*linearapi.Issue)

	for i := range issues {
		issue := &issues[i]
		if issue.Parent == nil {
			// No parent - this is a top-level issue
			topLevel = append(topLevel, issue)
		} else if _, parentInList := idToIssue[issue.Parent.ID]; parentInList {
			// Parent is in our list - group under parent
			childrenByParent[issue.Parent.ID] = append(childrenByParent[issue.Parent.ID], issue)
		} else {
			// Orphan sub-issue (parent not in list) - treat as top-level with marker
			topLevel = append(topLevel, issue)
		}
	}

	// Build rows
	var rows []IssueRow

	for _, issue := range topLevel {
		// Check if this issue has children in our list
		children := childrenByParent[issue.ID]
		hasChildren := len(children) > 0 || len(issue.Children) > 0
		isExpanded := expanded[issue.ID]

		rows = append(rows, IssueRow{
			IssueID:     issue.ID,
			Level:       0,
			IsParent:    hasChildren,
			HasChildren: hasChildren,
			IsExpanded:  isExpanded,
		})

		// If expanded, add children
		if hasChildren && isExpanded {
			// Use children from our fetched list if available
			if len(children) > 0 {
				// Sort children by identifier for consistent ordering
				sort.Slice(children, func(i, j int) bool {
					return children[i].Identifier < children[j].Identifier
				})

				for _, child := range children {
					childHasChildren := len(child.Children) > 0
					childExpanded := expanded[child.ID]

					rows = append(rows, IssueRow{
						IssueID:     child.ID,
						Level:       1,
						IsParent:    childHasChildren,
						HasChildren: childHasChildren,
						IsExpanded:  childExpanded,
					})
				}
			}
		}
	}

	return rows, idToIssue
}

// ToggleExpanded toggles the expanded state for an issue.
// Returns the new expanded state.
func ToggleExpanded(expanded map[string]bool, issueID string) bool {
	newState := !expanded[issueID]
	expanded[issueID] = newState
	return newState
}

// CollapseAll sets all issues to collapsed state.
func CollapseAll(expanded map[string]bool) {
	for k := range expanded {
		delete(expanded, k)
	}
}

// ExpandAll expands all parent issues.
func ExpandAll(expanded map[string]bool, issues []linearapi.Issue) {
	for _, issue := range issues {
		if len(issue.Children) > 0 || issue.Parent == nil {
			// Expand issues that have children
			expanded[issue.ID] = true
		}
	}
}

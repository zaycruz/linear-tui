package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// markdownRenderer is a shared glamour renderer for markdown content.
var markdownRenderer *glamour.TermRenderer

// initMarkdownRenderer initializes the glamour markdown renderer.
func initMarkdownRenderer() {
	var err error
	markdownRenderer, err = glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Fallback: create a basic renderer if custom style fails
		markdownRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
		)
	}
}

// renderMarkdown renders markdown content using glamour.
// Falls back to plain text if rendering fails.
func renderMarkdown(content string) string {
	if markdownRenderer == nil {
		initMarkdownRenderer()
	}

	rendered, err := markdownRenderer.Render(content)
	if err != nil {
		// Fallback to plain text on error
		return content
	}

	// Trim extra whitespace that glamour may add
	return strings.TrimSpace(rendered)
}

// buildDetailsView creates and configures the details view with separate description and comments sections.
func (a *App) buildDetailsView() *tview.Flex {
	// Create description/metadata view (top section, scrollable)
	a.detailsDescriptionView = tview.NewTextView()
	a.detailsDescriptionView.SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle(" Details ").
		SetTitleColor(a.theme.Foreground).
		SetBorderColor(a.theme.Border).
		SetBackgroundColor(tcell.ColorDefault)
	padding := a.density.DetailsPadding
	a.detailsDescriptionView.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Create comments view (bottom section, scrollable, fixed height)
	a.detailsCommentsView = tview.NewTextView()
	a.detailsCommentsView.SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle(" Comments ").
		SetTitleColor(a.theme.Foreground).
		SetBorderColor(a.theme.Border).
		SetBackgroundColor(tcell.ColorDefault)
	a.detailsCommentsView.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Create flex layout; comments are added conditionally after issue selection.
	detailsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	a.detailsView = detailsFlex
	a.setDetailsCommentsVisibility(false)

	return a.detailsView
}

// setDetailsCommentsVisibility rebuilds the details layout to show or hide comments.
func (a *App) setDetailsCommentsVisibility(showComments bool) {
	if a.detailsView == nil || a.detailsDescriptionView == nil || a.detailsCommentsView == nil {
		return
	}
	if a.detailsCommentsVisible == showComments && a.detailsView.GetItemCount() > 0 {
		return
	}

	a.detailsView.Clear().
		AddItem(a.detailsDescriptionView, 0, 3, true)
	if showComments {
		a.detailsView.AddItem(a.detailsCommentsView, 0, 2, false)
	}

	a.detailsCommentsVisible = showComments
	if !showComments {
		a.focusedDetailsView = false
	}
}

// updateDetailsView updates the details view with the selected issue.
func (a *App) updateDetailsView() {
	a.issuesMu.RLock()
	selectedIssue := a.selectedIssue
	a.issuesMu.RUnlock()
	hasComments := selectedIssue != nil && len(selectedIssue.Comments) > 0
	a.setDetailsCommentsVisibility(hasComments)
	if selectedIssue == nil {
		// Show cycle details when a cycle is selected and no issue is focused
		if a.selectedNavigation != nil && a.selectedNavigation.IsCycle && a.selectedCycle != nil {
			a.updateCycleDetailsView(a.selectedCycle)
			return
		}
		a.detailsDescriptionView.SetTitle(" Details ")
		a.detailsDescriptionView.SetText(fmt.Sprintf("%sNo issue selected. Select an issue from the list to view details.[-]", a.themeTags.SecondaryText))
		a.detailsCommentsView.SetText("")
		if a.focusedPane == FocusDetails && !a.detailsCommentsVisible {
			a.updateFocus()
		}
		return
	}

	issue := selectedIssue
	a.detailsDescriptionView.SetTitle(" Details ")

	// Helper to colorize keys
	keyColor := a.themeTags.SecondaryText
	valColor := a.themeTags.Foreground
	accentColor := a.themeTags.Accent
	dividerColor := a.themeTags.Border
	sectionGap := a.density.DetailsSectionGap

	// ===== Update Description/Metadata View =====
	var headerLines []string

	// Issue header info with styling
	headerLines = append(headerLines, fmt.Sprintf("%s%s[-]", accentColor, issue.Identifier))
	headerLines = append(headerLines, fmt.Sprintf("[b]%s%s[-]", valColor, issue.Title))
	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}

	// Metadata grid simulation
	headerLines = append(headerLines, fmt.Sprintf("%sState:[-]      %s%s[-]", keyColor, valColor, issue.State))

	assignee := "Unassigned"
	if issue.Assignee != "" {
		assignee = issue.Assignee
	}
	headerLines = append(headerLines, fmt.Sprintf("%sAssignee:[-]   %s%s[-]", keyColor, valColor, assignee))

	headerLines = append(headerLines, fmt.Sprintf("%sPriority:[-]   %s%s[-]", keyColor, valColor, formatPriorityLabel(issue.Priority)))

	// Estimate
	if issue.Estimate != nil {
		headerLines = append(headerLines, fmt.Sprintf("%sEstimate:[-]   %s%.0f pts[-]", keyColor, valColor, *issue.Estimate))
	}

	// Due date with overdue highlight
	if issue.DueDate != "" {
		dueDateDisplay := issue.DueDate
		dueDateColor := valColor
		// Check if overdue
		if t, err := time.Parse("2006-01-02", issue.DueDate); err == nil {
			if time.Now().After(t.AddDate(0, 0, 1)) { // past end of due date day
				dueDateColor = a.themeTags.Error
				dueDateDisplay = issue.DueDate + " (overdue)"
			}
		}
		headerLines = append(headerLines, fmt.Sprintf("%sDue Date:[-]   %s%s[-]", keyColor, dueDateColor, dueDateDisplay))
	}

	// Labels
	labelsText := "No labels"
	if len(issue.Labels) > 0 {
		labelNames := make([]string, len(issue.Labels))
		for i, lbl := range issue.Labels {
			labelNames[i] = lbl.Name
		}
		labelsText = strings.Join(labelNames, ", ")
	}
	headerLines = append(headerLines, fmt.Sprintf("%sLabels:[-]     %s%s[-]", keyColor, valColor, labelsText))

	// Parent issue (if this is a sub-issue)
	if issue.Parent != nil {
		parentText := fmt.Sprintf("%s - %s", issue.Parent.Identifier, issue.Parent.Title)
		headerLines = append(headerLines, fmt.Sprintf("%sParent:[-]     %s%s[-]", keyColor, accentColor, parentText))
	}

	// Sub-issues (if this is a parent issue)
	if len(issue.Children) > 0 {
		for i := 0; i < sectionGap; i++ {
			headerLines = append(headerLines, "")
		}
		headerLines = append(headerLines, fmt.Sprintf("%sSub-issues:[-] %s%d items[-]", keyColor, valColor, len(issue.Children)))
		for _, child := range issue.Children {
			// Show child identifier, state, and title
			childLine := fmt.Sprintf("  %s└─[-] %s%s[-] %s[%s][-] %s%s[-]",
				keyColor,
				accentColor, child.Identifier,
				keyColor, child.State,
				valColor, child.Title)
			headerLines = append(headerLines, childLine)
		}
	}

	// GitHub PR links (from attachments)
	githubAttachments := filterGitHubAttachments(issue.Attachments)
	if len(githubAttachments) > 0 {
		for i := 0; i < sectionGap; i++ {
			headerLines = append(headerLines, "")
		}
		headerLines = append(headerLines, fmt.Sprintf("%sPull Requests:[-]", keyColor))
		for _, att := range githubAttachments {
			title := att.Title
			if title == "" {
				title = att.URL
			}
			headerLines = append(headerLines, fmt.Sprintf("  %s• %s[-] %s%s[-]", keyColor, title, accentColor, att.URL))
		}
	}

	// Relations
	if len(issue.Relations) > 0 {
		for i := 0; i < sectionGap; i++ {
			headerLines = append(headerLines, "")
		}
		headerLines = append(headerLines, fmt.Sprintf("%sRelations:[-]", keyColor))
		for _, rel := range issue.Relations {
			icon := relationIcon(rel.Type)
			headerLines = append(headerLines, fmt.Sprintf("  %s%s %s[-] %s%s[-] %s%s[-] %s[%s][-]",
				keyColor, icon, rel.Type,
				accentColor, rel.RelatedIssue.Identifier,
				valColor, rel.RelatedIssue.Title,
				keyColor, rel.RelatedState))
		}
	}

	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}
	headerLines = append(headerLines, fmt.Sprintf("%s────────────────────────────────────────[-]", dividerColor))
	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}

	// Set header first, then append description via ANSIWriter
	a.detailsDescriptionView.Clear()
	a.detailsDescriptionView.SetText(strings.Join(headerLines, "\n"))
	writer := tview.ANSIWriter(a.detailsDescriptionView)

	// Description
	if issue.Description != "" {
		_, _ = fmt.Fprintf(writer, "%sDescription:[-]\n\n", keyColor)

		// Render description as markdown and write through ANSIWriter
		// ANSIWriter translates ANSI escape codes to tview color tags
		renderedDesc := renderMarkdown(issue.Description)
		_, _ = fmt.Fprint(writer, renderedDesc)
	} else {
		_, _ = fmt.Fprintf(writer, "%sNo description available[-]", keyColor)
	}

	a.detailsDescriptionView.ScrollToBeginning()

	// ===== Update Comments View =====
	a.detailsCommentsView.Clear()
	commentsWriter := tview.ANSIWriter(a.detailsCommentsView)

	if len(issue.Comments) > 0 {
		_, _ = fmt.Fprintf(commentsWriter, "%sComments:[-] (%d)\n\n", keyColor, len(issue.Comments))

		for i, comment := range issue.Comments {
			// Comment header: author and timestamp
			authorDisplay := comment.Author.DisplayName
			if authorDisplay == "" {
				authorDisplay = comment.Author.Name
			}
			if comment.Author.IsMe {
				authorDisplay = fmt.Sprintf("%s (me)", authorDisplay)
			}

			// Format timestamp
			timeStr := comment.CreatedAt.Format("Jan 2, 2006 3:04 PM")
			if !comment.UpdatedAt.Equal(comment.CreatedAt) {
				timeStr += " (edited)"
			}

			_, _ = fmt.Fprintf(commentsWriter, "%s%s[-] %s%s[-]\n", accentColor, authorDisplay, keyColor, timeStr)
			_, _ = fmt.Fprint(commentsWriter, "\n")

			// Render comment body as markdown
			renderedComment := renderMarkdown(comment.Body)
			_, _ = fmt.Fprint(commentsWriter, renderedComment)

			// Add separator between comments (but not after the last one)
			if i < len(issue.Comments)-1 {
				_, _ = fmt.Fprint(commentsWriter, "\n\n")
				_, _ = fmt.Fprintf(commentsWriter, "%s────────────────────────────────────────[-]\n\n", dividerColor)
			}
		}
	} else {
		// Empty state for comments
		_, _ = fmt.Fprintf(commentsWriter, "%sNo comments yet.[-]", keyColor)
	}

	a.detailsCommentsView.ScrollToBeginning()
	if a.focusedPane == FocusDetails && !a.detailsCommentsVisible {
		a.updateFocus()
	}
}

// updateCycleDetailsView renders cycle metadata in the details panel.
// Called when a cycle node is selected in the navigation tree and no issue is focused.
func (a *App) updateCycleDetailsView(cycle *linearapi.Cycle) {
	if a.detailsDescriptionView == nil {
		return
	}
	a.setDetailsCommentsVisibility(false)

	keyColor := a.themeTags.SecondaryText
	valColor := a.themeTags.Foreground
	accentColor := a.themeTags.Accent
	dividerColor := a.themeTags.Border

	var lines []string

	lines = append(lines, fmt.Sprintf("%s%s[-]", accentColor, cycle.DisplayName()))
	lines = append(lines, "")

	if !cycle.StartsAt.IsZero() {
		lines = append(lines, fmt.Sprintf("%sDates:[-]     %s%s → %s[-]",
			keyColor, valColor,
			cycle.StartsAt.Format("Jan 2, 2006"),
			cycle.EndsAt.Format("Jan 2, 2006")))
	}

	lines = append(lines, fmt.Sprintf("%sProgress:[-]  %s%d%%[-]", keyColor, valColor, cycle.ProgressPercent()))

	// Burndown bar
	burndown := buildCycleBurndownText(cycle, 24)
	if burndown != "" {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s%s[-]", valColor, burndown))
	}

	// Issue breakdown by state
	a.issuesMu.RLock()
	issues := make([]linearapi.Issue, len(a.issues))
	copy(issues, a.issues)
	a.issuesMu.RUnlock()

	if len(issues) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s────────────────[-]", dividerColor))
		lines = append(lines, "")

		stateCounts := map[string]int{}
		stateOrder := []string{}
		seen := map[string]bool{}
		for _, iss := range issues {
			if !seen[iss.State] {
				stateOrder = append(stateOrder, iss.State)
				seen[iss.State] = true
			}
			stateCounts[iss.State]++
		}
		for _, st := range stateOrder {
			lines = append(lines, fmt.Sprintf("  %s%s[-]  %s%d[-]", keyColor, st, valColor, stateCounts[st]))
		}

		breakdown := buildAssigneeBreakdown(issues)
		if breakdown != "" {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("%s────────────────[-]", dividerColor))
			lines = append(lines, "")
			for _, part := range strings.Split(breakdown, "  |  ") {
				lines = append(lines, "  "+strings.TrimSpace(part))
			}
		}

		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s[K] open kanban view[-]", keyColor))
	}

	a.detailsDescriptionView.SetTitle(" Cycle ")
	a.detailsDescriptionView.SetText(strings.Join(lines, "\n"))
	a.detailsDescriptionView.ScrollToBeginning()
}

// filterGitHubAttachments returns attachments that are GitHub PR links.
func filterGitHubAttachments(attachments []linearapi.IssueAttachment) []linearapi.IssueAttachment {
	var result []linearapi.IssueAttachment
	for _, att := range attachments {
		if att.SourceType == "github" || strings.Contains(att.URL, "github.com/") {
			result = append(result, att)
		}
	}
	return result
}

// relationIcon returns an icon for a relation type.
func relationIcon(relType string) string {
	switch relType {
	case "blocks":
		return "⊃"
	case "blocked":
		return "⊂"
	case "duplicate":
		return "="
	case "duplicateOf":
		return "="
	default:
		return "~"
	}
}

// formatPriorityLabel returns a human-readable priority label.
func formatPriorityLabel(priority int) string {
	switch priority {
	case 1:
		return "Urgent"
	case 2:
		return "High"
	case 3:
		return "Medium"
	case 4:
		return "Low"
	default:
		return "None"
	}
}


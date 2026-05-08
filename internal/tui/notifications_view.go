package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// renderNotificationsTable renders notifications into a tview table.
func renderNotificationsTable(table *tview.Table, notifications []linearapi.Notification, theme Theme) {
	table.Clear()

	// Headers
	headerStyle := tcell.StyleDefault.
		Foreground(theme.HeaderText).
		Background(theme.HeaderBg).
		Bold(true)

	table.SetCell(0, 0, tview.NewTableCell("Type").
		SetStyle(headerStyle).SetSelectable(false).SetExpansion(2))
	table.SetCell(0, 1, tview.NewTableCell("Issue").
		SetStyle(headerStyle).SetSelectable(false).SetExpansion(1))
	table.SetCell(0, 2, tview.NewTableCell("Title").
		SetStyle(headerStyle).SetSelectable(false).SetExpansion(4))
	table.SetCell(0, 3, tview.NewTableCell("Actor").
		SetStyle(headerStyle).SetSelectable(false).SetExpansion(2))
	table.SetCell(0, 4, tview.NewTableCell("Time").
		SetStyle(headerStyle).SetSelectable(false).SetExpansion(2))

	if len(notifications) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("No notifications").
			SetTextColor(theme.SecondaryText).SetSelectable(false).SetExpansion(10))
		for col := 1; col <= 4; col++ {
			table.SetCell(1, col, tview.NewTableCell("").SetSelectable(false))
		}
		return
	}

	for i, n := range notifications {
		row := i + 1
		isUnread := n.ReadAt == ""

		textColor := theme.SecondaryText
		if isUnread {
			textColor = theme.Foreground
		}

		// Format type into readable label
		typeLabel := formatNotificationType(n.Type)
		typeColor := textColor
		if isUnread {
			typeColor = theme.Accent
		}

		// Format time
		timeStr := formatNotificationTime(n.CreatedAt)

		table.SetCell(row, 0, tview.NewTableCell(typeLabel).
			SetTextColor(typeColor).SetExpansion(2))
		table.SetCell(row, 1, tview.NewTableCell(n.IssueIdentifier).
			SetTextColor(textColor).SetExpansion(1))
		table.SetCell(row, 2, tview.NewTableCell(n.IssueTitle).
			SetTextColor(textColor).SetExpansion(4))
		table.SetCell(row, 3, tview.NewTableCell(n.ActorName).
			SetTextColor(theme.SecondaryText).SetExpansion(2))
		table.SetCell(row, 4, tview.NewTableCell(timeStr).
			SetTextColor(theme.SecondaryText).SetExpansion(2))
	}

	table.SetFixed(1, 0)
	if len(notifications) > 0 {
		table.Select(1, 0)
	}
}

// formatNotificationType converts a notification type to a readable label.
func formatNotificationType(t string) string {
	switch t {
	case "issueAssigned":
		return "Assigned"
	case "issueMention":
		return "Mentioned"
	case "commentMention":
		return "Comment mention"
	case "issueStatusChanged":
		return "Status changed"
	case "issueNewComment":
		return "New comment"
	default:
		// Convert camelCase to Title Case with spaces
		result := strings.ToLower(string(t[0])) + t[1:]
		return result
	}
}

// formatNotificationTime formats an RFC3339 time string to a relative/short display.
func formatNotificationTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

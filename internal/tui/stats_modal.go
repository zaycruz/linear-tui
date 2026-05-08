package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

const statsBarMaxWidth = 12

// StatsModal shows team throughput statistics.
type StatsModal struct {
	app     *App
	modal   *tview.Flex
	content *tview.TextView
}

// NewStatsModal creates a new stats modal.
func NewStatsModal(app *App) *StatsModal {
	sm := &StatsModal{app: app}

	sm.content = tview.NewTextView()
	sm.content.SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true).
		SetTextColor(app.theme.Foreground).
		SetBackgroundColor(app.theme.HeaderBg)

	helpView := tview.NewTextView()
	helpView.SetText("q / Esc: close")
	helpView.SetTextColor(app.theme.SecondaryText)
	helpView.SetBackgroundColor(app.theme.HeaderBg)

	inner := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(sm.content, 0, 1, true).
		AddItem(helpView, 1, 0, false)
	inner.SetBorder(true).
		SetTitle(" Team Stats ").
		SetTitleColor(app.theme.Accent).
		SetBorderColor(app.theme.Accent).
		SetBackgroundColor(app.theme.HeaderBg)

	sm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inner, 24, 0, true).
			AddItem(nil, 0, 1, false), 65, 0, true).
		AddItem(nil, 0, 1, false)
	sm.modal.SetBackgroundColor(app.theme.Background)

	return sm
}

// Show computes stats from issues and displays them.
func (sm *StatsModal) Show(issues []linearapi.Issue, teamName string) {
	text := renderTeamStats(issues, teamName)
	sm.content.Clear()
	sm.content.SetText(text)
	sm.content.ScrollToBeginning()

	sm.app.pages.AddPage("stats", sm.modal, true, true)
	sm.app.pages.SendToFront("stats")
	sm.app.app.SetFocus(sm.content)
}

// Hide dismisses the stats modal.
func (sm *StatsModal) Hide() {
	sm.app.pages.RemovePage("stats")
	sm.app.updateFocus()
}

// HandleKey handles keyboard input.
func (sm *StatsModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		sm.Hide()
		return nil
	case tcell.KeyRune:
		if event.Rune() == 'q' {
			sm.Hide()
			return nil
		}
	}
	return event
}

type periodStats struct {
	opened int
	closed int
}

// renderTeamStats builds the stats text from a list of issues.
func renderTeamStats(issues []linearapi.Issue, teamName string) string {
	now := time.Now()

	// Time boundaries
	thisWeekStart := now.AddDate(0, 0, -int(now.Weekday()))
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart
	thisMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var thisWeek, lastWeek, thisMonth periodStats
	stateCount := make(map[string]int)

	for _, issue := range issues {
		// Count by state for state breakdown
		stateCount[issue.State]++

		// "opened" = createdAt in window
		// "closed" = updatedAt in window and state type = completed/canceled
		isDone := false
		// We can't access stateType from Issue directly, check state name heuristics
		// completed state types usually contain "Done", "Completed", "Closed", "Cancelled"
		stateLower := strings.ToLower(issue.State)
		if strings.Contains(stateLower, "done") || strings.Contains(stateLower, "complet") ||
			strings.Contains(stateLower, "cancel") || strings.Contains(stateLower, "clos") {
			isDone = true
		}

		// This week
		if issue.CreatedAt.After(thisWeekStart) {
			thisWeek.opened++
		}
		if isDone && issue.UpdatedAt.After(thisWeekStart) {
			thisWeek.closed++
		}

		// Last week
		if issue.CreatedAt.After(lastWeekStart) && issue.CreatedAt.Before(lastWeekEnd) {
			lastWeek.opened++
		}
		if isDone && issue.UpdatedAt.After(lastWeekStart) && issue.UpdatedAt.Before(lastWeekEnd) {
			lastWeek.closed++
		}

		// This month
		if issue.CreatedAt.After(thisMonthStart) {
			thisMonth.opened++
		}
		if isDone && issue.UpdatedAt.After(thisMonthStart) {
			thisMonth.closed++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("─── Team Stats — %s ───\n\n", teamName))

	writeStatRow := func(label string, stats periodStats) {
		net := stats.closed - stats.opened
		netStr := fmt.Sprintf("net +%d", net)
		if net < 0 {
			netStr = fmt.Sprintf("net %d", net)
		} else if net == 0 {
			netStr = "net 0"
		}
		sb.WriteString(fmt.Sprintf("%-12s %3d opened   %3d closed   %s\n",
			label+":", stats.opened, stats.closed, netStr))
	}

	writeStatRow("This week", thisWeek)
	writeStatRow("Last week", lastWeek)
	writeStatRow("This month", thisMonth)

	// By state breakdown
	if len(stateCount) > 0 {
		sb.WriteString("\nBy state:\n")
		// Find max for normalization
		maxCount := 0
		for _, c := range stateCount {
			if c > maxCount {
				maxCount = c
			}
		}
		// Sort states by count descending
		type kv struct{ k string; v int }
		var pairs []kv
		for k, v := range stateCount {
			pairs = append(pairs, kv{k, v})
		}
		// Simple sort by count desc
		for i := 0; i < len(pairs)-1; i++ {
			for j := i + 1; j < len(pairs); j++ {
				if pairs[j].v > pairs[i].v {
					pairs[i], pairs[j] = pairs[j], pairs[i]
				}
			}
		}
		for _, p := range pairs {
			bar := buildVelocityBar(p.v, maxCount, statsBarMaxWidth)
			sb.WriteString(fmt.Sprintf("  %-20s %s  %d\n", p.k, bar, p.v))
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("─", 50)))
	sb.WriteString(fmt.Sprintf("Total issues in view: %d\n", len(issues)))

	return sb.String()
}

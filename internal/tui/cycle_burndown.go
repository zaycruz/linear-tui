package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// burndownBlocks maps a fraction (0.0–1.0) to a Unicode block character.
// Higher fraction = taller block.
var burndownBlocks = []rune{'░', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// renderBurndownBar renders a single-line ASCII bar chart from daily history arrays.
// completed and total are parallel slices of daily counts.
// width is the desired bar width in characters.
// Returns a string like: ░░░▂▃▄▅▆
func renderBurndownBar(completed, total []int, width int) string {
	if width <= 0 {
		width = 10
	}

	n := len(completed)
	if n == 0 || len(total) == 0 {
		return strings.Repeat(string(burndownBlocks[0]), width)
	}

	// Sample the history into width data points.
	var bar strings.Builder
	for i := 0; i < width; i++ {
		// Map bar position to history index.
		histIdx := i * (n - 1) / (width - 1)
		if width == 1 {
			histIdx = n - 1
		}
		if histIdx >= n {
			histIdx = n - 1
		}

		tot := 0
		if histIdx < len(total) {
			tot = total[histIdx]
		}
		comp := 0
		if histIdx < len(completed) {
			comp = completed[histIdx]
		}

		var fraction float64
		if tot > 0 {
			fraction = float64(comp) / float64(tot)
		}
		if fraction > 1.0 {
			fraction = 1.0
		}

		blockIdx := int(fraction * float64(len(burndownBlocks)-1))
		bar.WriteRune(burndownBlocks[blockIdx])
	}
	return bar.String()
}

// buildCycleBurndownText returns a single content line describing the cycle.
// Format: Cycle #1  May 11→25  ░░░▂▃▄▅▆  12/28 done  42%
func buildCycleBurndownText(cycle *linearapi.Cycle, barWidth int) string {
	if cycle == nil {
		return ""
	}

	name := cycle.DisplayName()

	dateRange := ""
	if !cycle.StartsAt.IsZero() && !cycle.EndsAt.IsZero() {
		dateRange = fmt.Sprintf("  %s→%s",
			cycle.StartsAt.Format("Jan 2"),
			cycle.EndsAt.Format("Jan 2"))
	}

	bar := renderBurndownBar(cycle.CompletedIssueCountHistory, cycle.IssueCountHistory, barWidth)

	// Use the last data point for done/total counts.
	done := 0
	tot := 0
	if n := len(cycle.CompletedIssueCountHistory); n > 0 {
		done = cycle.CompletedIssueCountHistory[n-1]
	}
	if n := len(cycle.IssueCountHistory); n > 0 {
		tot = cycle.IssueCountHistory[n-1]
	}

	pct := cycle.ProgressPercent()

	return fmt.Sprintf("%s%s  %s  %d/%d done  %d%%", name, dateRange, bar, done, tot, pct)
}

// buildBurndownPanel creates a tview.TextView showing the cycle burndown bar.
func buildBurndownPanel(app *App, cycle *linearapi.Cycle) *tview.TextView {
	tv := tview.NewTextView()
	tv.SetBorder(true).
		SetTitle(" Cycle Burndown ").
		SetTitleColor(app.theme.Accent).
		SetBorderColor(app.theme.Border).
		SetBackgroundColor(app.theme.Background)
	tv.SetTextColor(app.theme.Foreground)
	tv.SetDynamicColors(false)

	if cycle != nil {
		text := buildCycleBurndownText(cycle, 20)
		tv.SetText(text)
	}

	return tv
}

// buildAssigneeBreakdown builds a one-line summary of issues per assignee for the cycle.
// Format: Alex: 3 done  2 in-progress  |  Jordan: 1 done  4 in-progress  |  Unassigned: 2
func buildAssigneeBreakdown(issues []linearapi.Issue) string {
	type assigneeStats struct {
		done       int
		inProgress int
		other      int
	}
	stats := make(map[string]*assigneeStats)

	for _, issue := range issues {
		name := issue.Assignee
		if name == "" {
			name = "Unassigned"
		}
		if _, ok := stats[name]; !ok {
			stats[name] = &assigneeStats{}
		}
		stateLower := strings.ToLower(issue.State)
		if strings.Contains(stateLower, "done") || strings.Contains(stateLower, "complet") || strings.Contains(stateLower, "cancel") {
			stats[name].done++
		} else if strings.Contains(stateLower, "progress") || strings.Contains(stateLower, "review") {
			stats[name].inProgress++
		} else {
			stats[name].other++
		}
	}

	if len(stats) == 0 {
		return ""
	}

	// Sort names for stable output
	names := make([]string, 0, len(stats))
	for n := range stats {
		names = append(names, n)
	}
	sort.Strings(names)

	var parts []string
	for _, name := range names {
		s := stats[name]
		part := fmt.Sprintf("%s: %d done  %d in-progress", name, s.done, s.inProgress)
		if s.other > 0 {
			part += fmt.Sprintf("  %d other", s.other)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "  |  ")
}

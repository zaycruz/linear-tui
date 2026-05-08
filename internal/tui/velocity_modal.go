package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

const velocityBarWidth = 16

// VelocityModal shows a bar chart of issues completed per cycle.
type VelocityModal struct {
	app     *App
	modal   *tview.Flex
	content *tview.TextView
}

// NewVelocityModal creates a new velocity modal.
func NewVelocityModal(app *App) *VelocityModal {
	vm := &VelocityModal{app: app}

	vm.content = tview.NewTextView()
	vm.content.SetDynamicColors(true).
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
		AddItem(vm.content, 0, 1, true).
		AddItem(helpView, 1, 0, false)
	inner.SetBorder(true).
		SetTitle(" Velocity — Last 6 Cycles ").
		SetTitleColor(app.theme.Accent).
		SetBorderColor(app.theme.Accent).
		SetBackgroundColor(app.theme.HeaderBg)

	vm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inner, 20, 0, true).
			AddItem(nil, 0, 1, false), 70, 0, true).
		AddItem(nil, 0, 1, false)
	vm.modal.SetBackgroundColor(app.theme.Background)

	return vm
}

// Show displays the velocity modal using cycles from the app state.
func (vm *VelocityModal) Show(cycles []linearapi.Cycle, teamName string) {
	text := renderVelocityChart(cycles, teamName)
	vm.content.Clear()
	vm.content.SetText(text)
	vm.content.ScrollToBeginning()

	vm.app.pages.AddPage("velocity", vm.modal, true, true)
	vm.app.pages.SendToFront("velocity")
	vm.app.app.SetFocus(vm.content)
}

// Hide dismisses the velocity modal.
func (vm *VelocityModal) Hide() {
	vm.app.pages.RemovePage("velocity")
	vm.app.updateFocus()
}

// HandleKey handles keyboard input.
func (vm *VelocityModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		vm.Hide()
		return nil
	case tcell.KeyRune:
		if event.Rune() == 'q' {
			vm.Hide()
			return nil
		}
	}
	return event
}

// renderVelocityChart builds the ASCII bar chart string for velocity.
func renderVelocityChart(cycles []linearapi.Cycle, teamName string) string {
	if len(cycles) == 0 {
		return "No cycle data available."
	}

	// Take last 6 cycles
	start := 0
	if len(cycles) > 6 {
		start = len(cycles) - 6
	}
	last6 := cycles[start:]

	// Collect completed counts
	counts := make([]int, len(last6))
	for i, cyc := range last6 {
		n := len(cyc.CompletedIssueCountHistory)
		if n > 0 {
			counts[i] = cyc.CompletedIssueCountHistory[n-1]
		}
	}

	// Find max for normalization
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("─── Velocity — %s ───\n\n", teamName))

	for i, cyc := range last6 {
		name := cyc.DisplayName()
		dateRange := ""
		if !cyc.StartsAt.IsZero() && !cyc.EndsAt.IsZero() {
			dateRange = fmt.Sprintf(" %s–%s", cyc.StartsAt.Format("Jan 2"), cyc.EndsAt.Format("Jan 2"))
		}

		done := counts[i]
		bar := buildVelocityBar(done, maxCount, velocityBarWidth)

		current := ""
		if i == len(last6)-1 {
			current = " ← current"
		}
		sb.WriteString(fmt.Sprintf("%-10s%s  %s  %d done%s\n", name, dateRange, bar, done, current))
	}

	// Summary line
	sum := 0
	for _, c := range counts {
		sum += c
	}
	avg := 0
	if len(counts) > 0 {
		avg = sum / len(counts)
	}
	best := maxCount

	trend := "→"
	if len(counts) >= 2 {
		last := counts[len(counts)-1]
		prev := counts[len(counts)-2]
		if last > prev {
			trend = "↑"
		} else if last < prev {
			trend = "↓"
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("─", 50)))
	sb.WriteString(fmt.Sprintf("Avg: %d / cycle    Best: %d    Trend: %s\n", avg, best, trend))

	return sb.String()
}

// buildVelocityBar builds a filled/empty bar string.
func buildVelocityBar(value, max, width int) string {
	if max == 0 || width == 0 {
		return strings.Repeat("░", width)
	}
	filled := value * width / max
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

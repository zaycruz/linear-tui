package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// CycleKanbanModal is a full-screen overlay showing cycle issues as swim lanes by workflow state.
type CycleKanbanModal struct {
	app        *App
	root       *tview.Flex // vertical: title bar + columns
	titleBar   *tview.TextView
	colsFlex   *tview.Flex // horizontal: one column per state
	columns    []*kanbanCol
	focusedCol int
}

type kanbanCol struct {
	stateID   string
	stateName string
	header    *tview.TextView
	list      *tview.List
	outer     *tview.Flex // vertical: header(3) + list(0,1)
	issues    []linearapi.Issue
}

func NewCycleKanbanModal(a *App) *CycleKanbanModal {
	titleBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	titleBar.SetBackgroundColor(tcell.ColorDefault)
	titleBar.SetBorder(false)

	colsFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	colsFlex.SetBackgroundColor(tcell.ColorDefault)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(titleBar, 1, 0, false).
		AddItem(colsFlex, 0, 1, true)
	root.SetBorder(true).
		SetBorderColor(tcell.ColorDarkSlateGray).
		SetBackgroundColor(tcell.ColorDefault)

	m := &CycleKanbanModal{
		app:      a,
		root:     root,
		titleBar: titleBar,
		colsFlex: colsFlex,
	}
	return m
}

// Show rebuilds the kanban columns and displays the modal.
func (m *CycleKanbanModal) Show(cycle *linearapi.Cycle, issues []linearapi.Issue, states []linearapi.WorkflowState) {
	m.buildColumns(cycle, issues, states)
	m.focusedCol = 0
	m.updateFocusHighlight()
	m.app.pages.ShowPage("cycle_kanban")
	if len(m.columns) > 0 {
		m.app.app.SetFocus(m.columns[0].list)
	}
}

// Hide closes the modal.
func (m *CycleKanbanModal) Hide() {
	m.app.pages.HidePage("cycle_kanban")
	m.app.updateFocus()
}

// HandleKey processes keyboard input for the modal.
func (m *CycleKanbanModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		m.Hide()
		return nil
	case tcell.KeyLeft:
		m.moveFocus(-1)
		return nil
	case tcell.KeyRight:
		m.moveFocus(1)
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'h':
			m.moveFocus(-1)
			return nil
		case 'l':
			m.moveFocus(1)
			return nil
		case 'q':
			m.Hide()
			return nil
		}
	}
	return event
}

func (m *CycleKanbanModal) moveFocus(delta int) {
	if len(m.columns) == 0 {
		return
	}
	m.focusedCol = (m.focusedCol + delta + len(m.columns)) % len(m.columns)
	m.updateFocusHighlight()
	m.app.app.SetFocus(m.columns[m.focusedCol].list)
}

func (m *CycleKanbanModal) updateFocusHighlight() {
	for i, col := range m.columns {
		if i == m.focusedCol {
			col.header.SetTextColor(tcell.ColorAqua)
			col.list.SetBorderColor(tcell.ColorAqua)
		} else {
			col.header.SetTextColor(tcell.ColorGray)
			col.list.SetBorderColor(tcell.ColorDarkSlateGray)
		}
	}
}

func (m *CycleKanbanModal) buildColumns(cycle *linearapi.Cycle, issues []linearapi.Issue, states []linearapi.WorkflowState) {
	m.colsFlex.Clear()
	m.columns = nil

	// Order states by position, skip empty states
	ordered := make([]linearapi.WorkflowState, len(states))
	copy(ordered, states)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Position < ordered[j].Position
	})

	// Group issues by state name (Linear returns state name as string)
	byState := map[string][]linearapi.Issue{}
	for _, iss := range issues {
		byState[iss.State] = append(byState[iss.State], iss)
	}

	// Only create columns for states that have issues (or all states if few)
	for _, s := range ordered {
		stateIssues := byState[s.Name]
		col := m.makeColumn(s, stateIssues)
		m.columns = append(m.columns, col)
		m.colsFlex.AddItem(col.outer, 0, 1, false)
	}

	// Update title bar
	cycleName := "Cycle"
	if cycle != nil {
		cycleName = cycle.DisplayName()
		if !cycle.StartsAt.IsZero() {
			cycleName += fmt.Sprintf("  %s→%s",
				cycle.StartsAt.Format("Jan 2"),
				cycle.EndsAt.Format("Jan 2"))
		}
	}
	m.titleBar.SetText(fmt.Sprintf("[cyan]%s[-]  [darkgray]h/l: switch col  j/k: scroll  Esc: close[-]", cycleName))
	m.root.SetTitle(fmt.Sprintf(" Kanban: %s ", cycleName))
}

func (m *CycleKanbanModal) makeColumn(state linearapi.WorkflowState, issues []linearapi.Issue) *kanbanCol {
	stateColor := stateTypeColor(state.Type)

	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	header.SetBackgroundColor(tcell.ColorDefault)
	header.SetBorder(false)
	header.SetText(fmt.Sprintf("[%s]%s[-]  [darkgray]%d[-]", stateColor, strings.ToUpper(state.Name), len(issues)))

	list := tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorDarkSlateGray).
		SetSelectedTextColor(tcell.ColorWhite).
		SetSecondaryTextColor(tcell.ColorGray)
	list.SetBorder(true).
		SetBorderColor(tcell.ColorDarkSlateGray).
		SetBackgroundColor(tcell.ColorDefault)

	for _, iss := range issues {
		assignee := iss.Assignee
		if assignee == "" {
			assignee = "unassigned"
		}
		pri, _ := formatPriority(iss.Priority, LinearTheme)
		list.AddItem(
			fmt.Sprintf("%s %s", pri, iss.Identifier),
			truncate(iss.Title, 28)+" [darkgray]"+assignee+"[-]",
			0, nil,
		)
	}

	if len(issues) == 0 {
		list.AddItem("[darkgray]empty[-]", "", 0, nil)
	}

	outer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(list, 0, 1, true)
	outer.SetBackgroundColor(tcell.ColorDefault)

	return &kanbanCol{
		stateID:   state.ID,
		stateName: state.Name,
		header:    header,
		list:      list,
		outer:     outer,
		issues:    issues,
	}
}

func stateTypeColor(stateType string) string {
	switch stateType {
	case "completed":
		return "green"
	case "started":
		return "cyan"
	case "canceled":
		return "red"
	case "backlog":
		return "darkgray"
	default:
		return "white"
	}
}

func truncate(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}

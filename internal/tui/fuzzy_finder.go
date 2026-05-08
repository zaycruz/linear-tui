package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// FuzzyFinder is a full-screen overlay for searching all cached issues.
type FuzzyFinder struct {
	app             *App
	modal           *tview.Flex
	inputField      *tview.InputField
	resultList      *tview.List
	allIssues       []linearapi.Issue
	filteredIssues  []linearapi.Issue
	visible         bool
}

// NewFuzzyFinder creates a new fuzzy finder overlay.
func NewFuzzyFinder(app *App) *FuzzyFinder {
	ff := &FuzzyFinder{app: app}

	ff.inputField = tview.NewInputField().
		SetLabel("Search: ").
		SetLabelColor(app.theme.Accent).
		SetFieldBackgroundColor(app.theme.HeaderBg).
		SetFieldTextColor(app.theme.Foreground)
	ff.inputField.SetBackgroundColor(app.theme.HeaderBg)

	ff.resultList = tview.NewList().
		ShowSecondaryText(false).
		SetMainTextColor(app.theme.Foreground).
		SetSelectedBackgroundColor(app.theme.Accent).
		SetSelectedTextColor(app.theme.SelectionText).
		SetHighlightFullLine(true)
	ff.resultList.SetBackgroundColor(app.theme.HeaderBg)

	helpView := tview.NewTextView()
	helpView.SetText("Type to filter | ↑↓/j/k: navigate | Enter: jump to issue | Esc: close")
	helpView.SetTextColor(app.theme.SecondaryText)
	helpView.SetBackgroundColor(app.theme.HeaderBg)

	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ff.inputField, 3, 0, true).
		AddItem(ff.resultList, 0, 1, false).
		AddItem(helpView, 1, 0, false)
	content.SetBorder(true).
		SetTitle(" Fuzzy Issue Finder (Ctrl+P) ").
		SetTitleColor(app.theme.Accent).
		SetBorderColor(app.theme.Accent).
		SetBackgroundColor(app.theme.HeaderBg)

	ff.modal = tview.NewFlex().
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 2, 0, false).
			AddItem(content, 0, 1, true).
			AddItem(nil, 2, 0, false), 0, 1, true).
		AddItem(nil, 2, 0, false)
	ff.modal.SetBackgroundColor(app.theme.Background)

	// Wire input changes to filter
	ff.inputField.SetChangedFunc(func(text string) {
		ff.filter(text)
	})

	return ff
}

// Show opens the fuzzy finder, loading all cached issues.
func (ff *FuzzyFinder) Show() {
	ff.visible = true

	// Snapshot all current issues
	ff.app.issuesMu.RLock()
	all := make([]linearapi.Issue, len(ff.app.issues))
	copy(all, ff.app.issues)
	ff.app.issuesMu.RUnlock()
	ff.allIssues = all

	// Reset
	ff.inputField.SetText("")
	ff.filter("")

	ff.app.pages.AddPage("fuzzy_finder", ff.modal, true, true)
	ff.app.pages.SendToFront("fuzzy_finder")
	ff.app.app.SetFocus(ff.inputField)
}

// Hide dismisses the fuzzy finder.
func (ff *FuzzyFinder) Hide() {
	ff.visible = false
	ff.app.pages.RemovePage("fuzzy_finder")
	ff.app.updateFocus()
}

// filter filters the issue list by the query.
func (ff *FuzzyFinder) filter(query string) {
	query = strings.TrimSpace(strings.ToLower(query))

	ff.filteredIssues = ff.filteredIssues[:0]
	// Exact identifier matches first
	var exactMatches []linearapi.Issue
	var fuzzyMatches []linearapi.Issue

	for _, issue := range ff.allIssues {
		combined := strings.ToLower(issue.Identifier + " " + issue.Title)
		if query == "" || strings.Contains(combined, query) {
			if query != "" && strings.EqualFold(issue.Identifier, query) {
				exactMatches = append(exactMatches, issue)
			} else {
				fuzzyMatches = append(fuzzyMatches, issue)
			}
		}
	}
	ff.filteredIssues = append(exactMatches, fuzzyMatches...)

	ff.resultList.Clear()
	for _, issue := range ff.filteredIssues {
		text := fmt.Sprintf("%-12s  %s  [%s]", issue.Identifier, issue.Title, issue.State)
		ff.resultList.AddItem(text, "", 0, nil)
	}
	if len(ff.filteredIssues) > 0 {
		ff.resultList.SetCurrentItem(0)
	}
}

// HandleKey handles key events for the fuzzy finder.
func (ff *FuzzyFinder) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		ff.Hide()
		return nil
	case tcell.KeyEnter:
		idx := ff.resultList.GetCurrentItem()
		if idx >= 0 && idx < len(ff.filteredIssues) {
			selected := ff.filteredIssues[idx]
			ff.Hide()
			ff.jumpToIssue(selected)
		}
		return nil
	case tcell.KeyUp:
		idx := ff.resultList.GetCurrentItem()
		if idx > 0 {
			ff.resultList.SetCurrentItem(idx - 1)
		}
		return nil
	case tcell.KeyDown:
		idx := ff.resultList.GetCurrentItem()
		if idx < ff.resultList.GetItemCount()-1 {
			ff.resultList.SetCurrentItem(idx + 1)
		}
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'j':
			idx := ff.resultList.GetCurrentItem()
			if idx < ff.resultList.GetItemCount()-1 {
				ff.resultList.SetCurrentItem(idx + 1)
			}
			return nil
		case 'k':
			idx := ff.resultList.GetCurrentItem()
			if idx > 0 {
				ff.resultList.SetCurrentItem(idx - 1)
			}
			return nil
		}
	}
	// Let input field handle other keys
	return event
}

// jumpToIssue navigates to the selected issue.
func (ff *FuzzyFinder) jumpToIssue(issue linearapi.Issue) {
	// Try to select this issue in the visible tables
	ff.app.issuesMu.Lock()
	ff.app.selectedIssue = &issue
	ff.app.issuesMu.Unlock()

	// Select in the appropriate table
	if _, ok := ff.app.myIDToIssue[issue.ID]; ok {
		ff.app.activeIssuesSection = IssuesSectionMy
		renderIssuesTableModel(ff.app.myIssuesTable, ff.app.myIssueRows, ff.app.myIDToIssue, issue.ID, ff.app.theme)
	} else if _, ok := ff.app.otherIDToIssue[issue.ID]; ok {
		ff.app.activeIssuesSection = IssuesSectionOther
		renderIssuesTableModel(ff.app.otherIssuesTable, ff.app.otherIssueRows, ff.app.otherIDToIssue, issue.ID, ff.app.theme)
	}

	ff.app.focusedPane = FocusIssues
	ff.app.updateFocus()
	ff.app.onIssueSelected(issue)
}

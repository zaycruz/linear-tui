package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
	"github.com/roeyazroel/linear-tui/internal/logger"
)

// TriageModal manages rapid-fire issue grooming mode.
type TriageModal struct {
	app     *App
	modal   *tview.Flex
	header  *tview.TextView
	content *tview.TextView
	footer  *tview.TextView

	issues []linearapi.Issue
	index  int
	active bool
}

// NewTriageModal creates a new triage modal.
func NewTriageModal(app *App) *TriageModal {
	tm := &TriageModal{app: app}

	tm.header = tview.NewTextView()
	tm.header.SetDynamicColors(true).
		SetTextColor(app.theme.Accent).
		SetBackgroundColor(app.theme.HeaderBg)

	tm.content = tview.NewTextView()
	tm.content.SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetTextColor(app.theme.Foreground).
		SetBackgroundColor(app.theme.HeaderBg)

	tm.footer = tview.NewTextView()
	tm.footer.SetText("[s] Status  [p] Priority  [a] Assign  [e] Estimate  [→/n] Skip  [q] Done")
	tm.footer.SetTextColor(app.theme.SecondaryText)
	tm.footer.SetBackgroundColor(app.theme.HeaderBg)

	inner := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tm.header, 2, 0, false).
		AddItem(tm.content, 0, 1, true).
		AddItem(tm.footer, 1, 0, false)
	inner.SetBorder(true).
		SetTitle(" TRIAGE MODE ").
		SetTitleColor(app.theme.Accent).
		SetBorderColor(app.theme.Accent).
		SetBackgroundColor(app.theme.HeaderBg)

	tm.modal = tview.NewFlex().
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 2, 0, false).
			AddItem(inner, 0, 1, true).
			AddItem(nil, 2, 0, false), 0, 1, true).
		AddItem(nil, 2, 0, false)
	tm.modal.SetBackgroundColor(app.theme.Background)

	return tm
}

// Start filters backlog/unstarted issues and opens the triage overlay.
func (tm *TriageModal) Start(allIssues []linearapi.Issue) {
	var backlog []linearapi.Issue
	for _, issue := range allIssues {
		stateLower := strings.ToLower(issue.State)
		if strings.Contains(stateLower, "backlog") || strings.Contains(stateLower, "unstart") ||
			strings.Contains(stateLower, "todo") || issue.State == "" {
			backlog = append(backlog, issue)
		}
	}

	if len(backlog) == 0 {
		tm.app.updateStatusBarWithError(fmt.Errorf("no backlog issues to triage"))
		return
	}

	tm.issues = backlog
	tm.index = 0
	tm.active = true

	tm.app.pages.AddPage("triage", tm.modal, true, true)
	tm.app.pages.SendToFront("triage")
	tm.renderCurrentIssue()
	tm.app.app.SetFocus(tm.content)
}

// Hide closes the triage modal.
func (tm *TriageModal) Hide() {
	tm.active = false
	tm.app.pages.RemovePage("triage")
	tm.app.updateFocus()
}

// renderCurrentIssue updates modal content for the current issue.
func (tm *TriageModal) renderCurrentIssue() {
	if tm.index >= len(tm.issues) {
		tm.header.SetText("All issues triaged!")
		tm.content.SetText("Press q or Esc to exit.")
		return
	}

	issue := tm.issues[tm.index]
	progress := fmt.Sprintf("Issue %d of %d", tm.index+1, len(tm.issues))
	tm.header.SetText(fmt.Sprintf("%s  —  %s", progress, issue.Identifier))

	pLabel := priorityLabel(issue.Priority)
	assigneeStr := issue.Assignee
	if assigneeStr == "" {
		assigneeStr = "Unassigned"
	}
	estimateStr := "–"
	if issue.Estimate != nil {
		estimateStr = fmt.Sprintf("%.0f pts", *issue.Estimate)
	}

	text := fmt.Sprintf("%s\n\nPriority: %s   Assignee: %s   Estimate: %s   State: %s\n\n%s",
		issue.Title,
		pLabel,
		assigneeStr,
		estimateStr,
		issue.State,
		truncateText(issue.Description, 400),
	)
	tm.content.SetText(text)
	tm.content.ScrollToBeginning()
}

func priorityLabel(p int) string {
	switch p {
	case 1:
		return "! Urgent"
	case 2:
		return "↑ High"
	case 3:
		return "→ Medium"
	case 4:
		return "↓ Low"
	default:
		return "None"
	}
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// advance moves to the next issue.
func (tm *TriageModal) advance() {
	tm.index++
	tm.renderCurrentIssue()
}

// applyUpdate runs an issue update and advances to next issue.
func (tm *TriageModal) applyUpdate(issueID, issueIdentifier string, input linearapi.UpdateIssueInput) {
	go func() {
		ctx := context.Background()
		_, err := tm.app.api.UpdateIssue(ctx, input)
		tm.app.QueueUpdateDraw(func() {
			if err != nil {
				logger.ErrorWithErr(err, "tui.triage: update failed issue=%s", issueIdentifier)
			}
			tm.advance()
		})
	}()
}

// HandleKey handles keyboard input for triage mode.
func (tm *TriageModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if tm.index >= len(tm.issues) {
		// All triaged — only allow q/Esc to close
		switch event.Key() {
		case tcell.KeyEscape:
			tm.Hide()
			return nil
		case tcell.KeyRune:
			if event.Rune() == 'q' {
				tm.Hide()
				return nil
			}
		}
		return nil
	}

	issue := tm.issues[tm.index]
	issueID := issue.ID
	issueIdentifier := issue.Identifier

	switch event.Key() {
	case tcell.KeyEscape:
		tm.Hide()
		return nil
	case tcell.KeyRight:
		tm.advance()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			tm.Hide()
			return nil
		case 'n':
			tm.advance()
			return nil
		case 's':
			// Status picker — apply and advance
			tm.app.ShowStatusPicker(func(stateID string) {
				tm.applyUpdate(issueID, issueIdentifier, linearapi.UpdateIssueInput{
					ID:      issueID,
					StateID: &stateID,
				})
			})
			return nil
		case 'p':
			// Priority picker — apply and advance
			tm.app.ShowPriorityPicker(func(priority int) {
				pCopy := priority
				tm.applyUpdate(issueID, issueIdentifier, linearapi.UpdateIssueInput{
					ID:       issueID,
					Priority: &pCopy,
				})
			})
			return nil
		case 'a':
			// Assign picker — apply and advance
			tm.app.ShowUserPickerWithUnassign(func(userID string) {
				uCopy := userID
				tm.applyUpdate(issueID, issueIdentifier, linearapi.UpdateIssueInput{
					ID:         issueID,
					AssigneeID: &uCopy,
				})
			})
			return nil
		case 'e':
			// Estimate picker — apply and advance
			tm.app.ShowEstimatePicker(func(estimate *float64) {
				updateEstimate := estimate
				if estimate == nil {
					zero := 0.0
					updateEstimate = &zero
				}
				tm.applyUpdate(issueID, issueIdentifier, linearapi.UpdateIssueInput{
					ID:       issueID,
					Estimate: updateEstimate,
				})
			})
			return nil
		}
	}
	return event
}

package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DueDateModal manages the due date input form overlay.
type DueDateModal struct {
	app          *App
	modal        *tview.Flex
	form         *tview.Form
	dateField    *tview.InputField
	issueID      string
	currentDate  string
	onUpdate     func(issueID, date string)
}

// NewDueDateModal creates a new due date modal.
func NewDueDateModal(app *App) *DueDateModal {
	ddm := &DueDateModal{
		app: app,
	}

	// Create form
	ddm.form = tview.NewForm()
	ddm.form.SetBackgroundColor(app.theme.HeaderBg)
	ddm.form.SetFieldBackgroundColor(app.theme.InputBg)
	ddm.form.SetFieldTextColor(app.theme.Foreground)
	ddm.form.SetButtonBackgroundColor(app.theme.Accent)
	ddm.form.SetButtonTextColor(app.theme.SelectionText)
	ddm.form.SetLabelColor(app.theme.Foreground)
	ddm.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ddm.Hide()
			return nil
		}
		return event
	})

	// Add date field
	ddm.dateField = tview.NewInputField()
	ddm.dateField.SetLabel("Due Date (YYYY-MM-DD or 'clear'): ")
	ddm.dateField.SetFieldWidth(20)
	ddm.dateField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ddm.Hide()
			return nil
		}
		return event
	})
	ddm.form.AddFormItem(ddm.dateField)

	// Add buttons
	ddm.form.AddButton("Set", func() {
		date := ddm.dateField.GetText()
		ddm.Hide()
		if ddm.onUpdate != nil && ddm.issueID != "" {
			ddm.onUpdate(ddm.issueID, date)
		}
	})
	ddm.form.AddButton("Cancel", func() {
		ddm.Hide()
	})

	// Create title
	titleView := tview.NewTextView()
	titleView.SetText("Set Due Date")
	titleView.SetTextColor(app.theme.Accent)
	titleView.SetBackgroundColor(app.theme.HeaderBg)

	// Help text
	helpView := tview.NewTextView()
	helpView.SetText("Enter YYYY-MM-DD to set, or 'clear' to remove the due date.")
	helpView.SetTextColor(app.theme.SecondaryText)
	helpView.SetBackgroundColor(app.theme.HeaderBg)

	// Build modal content
	modalContent := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(titleView, 1, 0, false).
		AddItem(helpView, 1, 0, false).
		AddItem(ddm.form, 0, 1, true)
	modalContent.Box = tview.NewBox().SetBackgroundColor(app.theme.HeaderBg)
	modalContent.SetBackgroundColor(app.theme.HeaderBg).
		SetBorder(true).
		SetBorderColor(app.theme.Accent).
		SetTitle(" Set Due Date ").
		SetTitleColor(app.theme.Foreground)
	padding := app.density.ModalPadding
	modalContent.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Center the modal on screen
	ddm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modalContent, 10, 0, true).
			AddItem(nil, 0, 1, false), 70, 0, true).
		AddItem(nil, 0, 1, false)
	ddm.modal.SetBackgroundColor(app.theme.Background)

	return ddm
}

// Show displays the due date modal.
func (ddm *DueDateModal) Show(issueID, currentDate string, onUpdate func(issueID, date string)) {
	ddm.issueID = issueID
	ddm.currentDate = currentDate
	ddm.onUpdate = onUpdate

	// Pre-fill with current date if set
	ddm.dateField.SetText(currentDate)

	ddm.app.pages.AddPage("due_date", ddm.modal, true, true)
	ddm.app.pages.SendToFront("due_date")
	ddm.app.app.SetFocus(ddm.form)
}

// Hide hides the due date modal.
func (ddm *DueDateModal) Hide() {
	ddm.app.pages.RemovePage("due_date")
	ddm.app.updateFocus()
}

// HandleKey handles keyboard input for the due date modal.
func (ddm *DueDateModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		ddm.Hide()
		return nil
	}
	return event
}

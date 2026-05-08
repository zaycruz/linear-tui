package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RelationModal manages the create relation form overlay.
type RelationModal struct {
	app              *App
	modal            *tview.Flex
	form             *tview.Form
	issueIDField     *tview.InputField
	relationTypeField *tview.DropDown
	issueID          string
	onCreate         func(issueID, relatedIssueID, relationType string)
}

// NewRelationModal creates a new relation modal.
func NewRelationModal(app *App) *RelationModal {
	rm := &RelationModal{
		app: app,
	}

	// Create form
	rm.form = tview.NewForm()
	rm.form.SetBackgroundColor(app.theme.HeaderBg)
	rm.form.SetFieldBackgroundColor(app.theme.InputBg)
	rm.form.SetFieldTextColor(app.theme.Foreground)
	rm.form.SetButtonBackgroundColor(app.theme.Accent)
	rm.form.SetButtonTextColor(app.theme.SelectionText)
	rm.form.SetLabelColor(app.theme.Foreground)
	rm.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			rm.Hide()
			return nil
		}
		return event
	})

	// Add related issue ID field
	rm.issueIDField = tview.NewInputField()
	rm.issueIDField.SetLabel("Related Issue ID: ")
	rm.issueIDField.SetFieldWidth(30)
	rm.issueIDField.SetPlaceholder("e.g. ABC-123")
	rm.issueIDField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			rm.Hide()
			return nil
		}
		return event
	})
	rm.form.AddFormItem(rm.issueIDField)

	// Add relation type dropdown
	rm.relationTypeField = tview.NewDropDown()
	rm.relationTypeField.SetLabel("Relation Type: ")
	rm.relationTypeField.SetOptions([]string{"related", "blocks", "blocked", "duplicate", "duplicateOf"}, nil)
	rm.relationTypeField.SetCurrentOption(0)
	rm.relationTypeField.SetFieldBackgroundColor(app.theme.InputBg)
	rm.relationTypeField.SetFieldTextColor(app.theme.Foreground)
	rm.form.AddFormItem(rm.relationTypeField)

	// Add buttons
	rm.form.AddButton("Create", func() {
		relatedID := rm.issueIDField.GetText()
		_, relationType := rm.relationTypeField.GetCurrentOption()
		rm.Hide()
		if rm.onCreate != nil && rm.issueID != "" && relatedID != "" && relationType != "" {
			rm.onCreate(rm.issueID, relatedID, relationType)
		}
	})
	rm.form.AddButton("Cancel", func() {
		rm.Hide()
	})

	// Create title
	titleView := tview.NewTextView()
	titleView.SetText("Create Issue Relation")
	titleView.SetTextColor(app.theme.Accent)
	titleView.SetBackgroundColor(app.theme.HeaderBg)

	// Build modal content
	modalContent := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(titleView, 1, 0, false).
		AddItem(rm.form, 0, 1, true)
	modalContent.Box = tview.NewBox().SetBackgroundColor(app.theme.HeaderBg)
	modalContent.SetBackgroundColor(app.theme.HeaderBg).
		SetBorder(true).
		SetBorderColor(app.theme.Accent).
		SetTitle(" Create Relation ").
		SetTitleColor(app.theme.Foreground)
	padding := app.density.ModalPadding
	modalContent.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Center the modal on screen
	rm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(modalContent, 12, 0, true).
			AddItem(nil, 0, 1, false), 60, 0, true).
		AddItem(nil, 0, 1, false)
	rm.modal.SetBackgroundColor(app.theme.Background)

	return rm
}

// Show displays the relation modal.
func (rm *RelationModal) Show(issueID string, onCreate func(issueID, relatedIssueID, relationType string)) {
	rm.issueID = issueID
	rm.onCreate = onCreate
	rm.issueIDField.SetText("")

	rm.app.pages.AddPage("relation", rm.modal, true, true)
	rm.app.pages.SendToFront("relation")
	rm.app.app.SetFocus(rm.form)
}

// Hide hides the relation modal.
func (rm *RelationModal) Hide() {
	rm.app.pages.RemovePage("relation")
	rm.app.updateFocus()
}

// HandleKey handles keyboard input for the relation modal.
func (rm *RelationModal) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEscape {
		rm.Hide()
		return nil
	}
	return event
}

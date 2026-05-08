package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/logger"
)

// promptSaveView opens a small input modal to name and save the current view.
func (a *App) promptSaveView() {
	inputField := tview.NewInputField().
		SetLabel("View name: ").
		SetLabelColor(a.theme.Accent).
		SetFieldBackgroundColor(a.theme.HeaderBg).
		SetFieldTextColor(a.theme.Foreground)
	inputField.SetBackgroundColor(a.theme.HeaderBg)

	helpText := tview.NewTextView()
	helpText.SetText("Enter: save | Esc: cancel")
	helpText.SetTextColor(a.theme.SecondaryText)
	helpText.SetBackgroundColor(a.theme.HeaderBg)

	inner := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 3, 0, true).
		AddItem(helpText, 1, 0, false)
	inner.SetBorder(true).
		SetTitle(" Save View ").
		SetTitleColor(a.theme.Accent).
		SetBorderColor(a.theme.Accent).
		SetBackgroundColor(a.theme.HeaderBg)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inner, 7, 0, true).
			AddItem(nil, 0, 1, false), 50, 0, true).
		AddItem(nil, 0, 1, false)
	modal.SetBackgroundColor(a.theme.Background)

	a.pages.AddPage("save_view_prompt", modal, true, true)
	a.pages.SendToFront("save_view_prompt")
	a.app.SetFocus(inputField)

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			name := strings.TrimSpace(inputField.GetText())
			a.pages.RemovePage("save_view_prompt")
			a.updateFocus()
			if name == "" {
				return
			}
			// Build the saved view from current filter state
			view := SavedView{Name: name}
			if a.filterAssigneeMe {
				view.Assignee = "me"
			}
			if a.filterLabelID != "" {
				view.LabelID = a.filterLabelID
			}
			if a.selectedNavigation != nil && a.selectedNavigation.IsStatus {
				view.State = a.selectedNavigation.StateName
			}

			// Add or update
			found := false
			for i, sv := range a.savedViews {
				if sv.Name == name {
					a.savedViews[i] = view
					found = true
					break
				}
			}
			if !found {
				a.savedViews = append(a.savedViews, view)
			}

			if err := saveSavedViews(a.savedViews); err != nil {
				logger.ErrorWithErr(err, "tui.saved_views: failed to save views")
				a.updateStatusBarWithError(err)
				return
			}
			logger.Info("tui.saved_views: saved view name=%s", name)
			a.statusBar.SetText(fmt.Sprintf("%sSaved view: %s[-]", a.themeTags.Accent, name))
			a.rebuildNavigationTreeWithSavedViews()

		case tcell.KeyEscape:
			a.pages.RemovePage("save_view_prompt")
			a.updateFocus()
		}
	})
}

// promptDeleteView shows a picker to delete a saved view.
func (a *App) promptDeleteView() {
	if len(a.savedViews) == 0 {
		a.updateStatusBarWithError(fmt.Errorf("no saved views to delete"))
		return
	}

	items := make([]PickerItem, 0, len(a.savedViews))
	for _, sv := range a.savedViews {
		items = append(items, PickerItem{ID: sv.Name, Label: sv.Name})
	}

	a.pickerActive = true
	a.pickerModal.Show("Delete Saved View", items, func(item PickerItem) {
		a.pickerActive = false
		// Remove the view with matching name
		newViews := make([]SavedView, 0, len(a.savedViews))
		for _, sv := range a.savedViews {
			if sv.Name != item.ID {
				newViews = append(newViews, sv)
			}
		}
		a.savedViews = newViews
		if err := saveSavedViews(a.savedViews); err != nil {
			logger.ErrorWithErr(err, "tui.saved_views: failed to delete view name=%s", item.ID)
			a.updateStatusBarWithError(err)
			return
		}
		logger.Info("tui.saved_views: deleted view name=%s", item.ID)
		a.statusBar.SetText(fmt.Sprintf("%sDeleted view: %s[-]", a.themeTags.Accent, item.ID))
		a.rebuildNavigationTreeWithSavedViews()
	})
}

// rebuildNavigationTreeWithSavedViews triggers a navigation tree rebuild to reflect saved view changes.
func (a *App) rebuildNavigationTreeWithSavedViews() {
	// Reload teams and rebuild the tree
	go func() {
		ctx := context.Background()
		teams, err := a.cache.GetTeams(ctx)
		if err != nil {
			return
		}
		a.app.QueueUpdateDraw(func() {
			a.rebuildNavigationTree(teams)
		})
	}()
}

// applySavedView applies a saved view's filters and refreshes issues.
func (a *App) applySavedView(sv SavedView) {
	// Reset previous filters
	a.filterAssigneeMe = false
	a.filterLabelID = ""
	a.filterLabelName = ""

	if sv.Assignee == "me" {
		a.filterAssigneeMe = true
	}
	if sv.LabelID != "" {
		a.filterLabelID = sv.LabelID
		a.filterLabelName = sv.Name + "/label"
	}

	a.updateStatusBar()
	go a.refreshIssues()
}

package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/roeyazroel/linear-tui/internal/agents"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
	"github.com/roeyazroel/linear-tui/internal/logger"
)

// FormatShortcut returns a human-readable string for a shortcut.
func FormatShortcut(r rune) string {
	if r == 0 {
		return ""
	}
	return strings.ToUpper(string(r))
}

// Command represents a command that can be executed from the palette.
type Command struct {
	ID              string
	Title           string
	Keywords        []string
	ShortcutRune    rune   // The rune for the keyboard shortcut (e.g., 'r' for refresh)
	ShortcutDisplay string // Custom display text for shortcut (e.g., "/" or "Esc"), overrides ShortcutRune display
	Run             func(a *App)
}

// CommandContext provides context for command execution.
type CommandContext struct {
	SelectedIssue *linearapi.Issue
}

// handleAskAgent handles the ask agent command.
func handleAskAgent(a *App) {
	issue := a.GetSelectedIssue()
	if issue == nil {
		a.updateStatusBarWithError(fmt.Errorf("no issue selected"))
		return
	}

	if a.agentPromptModal == nil {
		a.agentPromptModal = NewAgentPromptModal(a)
	}
	if a.agentOutputModal == nil {
		a.agentOutputModal = NewAgentOutputModal(a)
	}
	if a.agentRunner == nil {
		a.agentRunner = agents.NewRunner()
	}

	issueID := issue.ID
	a.agentPromptModal.Show(func(prompt string, workspace string) {
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			return
		}
		workspace = strings.TrimSpace(workspace)

		go func() {
			fetchIssue := a.fetchIssueByID
			if fetchIssue == nil {
				fetchIssue = a.api.FetchIssueByID
			}

			fullIssue, err := fetchIssue(context.Background(), issueID)
			if err != nil {
				logger.ErrorWithErr(err, "tui.commands: failed to fetch issue for agent issue_id=%s", issueID)
				a.QueueUpdateDraw(func() {
					a.updateStatusBarWithError(err)
				})
				return
			}

			issueContext := agents.BuildIssueContext(fullIssue)
			runner := a.agentRunner

			selected, err := agents.ProviderForKey(a.config.AgentProvider, runner.LookPath)
			if err != nil {
				logger.Error("tui.commands: invalid agent provider provider=%s", a.config.AgentProvider)
				a.QueueUpdateDraw(func() {
					a.updateStatusBarWithError(err)
				})
				return
			}

			if _, ok := selected.ResolveBinary(); !ok {
				logger.Error("tui.commands: agent binary not found provider=%s", selected.Name())
				a.QueueUpdateDraw(func() {
					a.updateStatusBarWithError(fmt.Errorf("agent binary not found for %s", selected.Name()))
				})
				return
			}

			options := agents.AgentRunOptions{
				Workspace: workspace,
				Model:     strings.TrimSpace(a.config.AgentModel),
				Sandbox:   strings.TrimSpace(a.config.AgentSandbox),
			}

			ctx, cancel := context.WithCancel(context.Background())
			a.QueueUpdateDraw(func() {
				title := fmt.Sprintf(" %s Output ", selected.Name())
				a.agentOutputModal.Show(title, cancel)
				a.agentOutputModal.AppendLine(fmt.Sprintf("Starting %s agent run...", selected.Name()))
			})

			runErr := runner.Run(ctx, selected, prompt, issueContext, options, func(event agents.AgentEvent) {
				a.agentOutputModal.AppendEvent(event)
			}, func(line string) {
				a.agentOutputModal.AppendRawLine(line)
			}, func(runErr error) {
				a.agentOutputModal.AppendLine(fmt.Sprintf("error: %v", runErr))
			})

			a.agentOutputModal.StopSpinner()

			if runErr != nil {
				a.QueueUpdateDraw(func() {
					a.agentOutputModal.AppendLine(fmt.Sprintf("error: %v", runErr))
				})
				return
			}

			a.agentOutputModal.AppendLine("Agent run completed.")
		}()
	})
}

// DefaultCommands returns the default set of commands for the palette.
func DefaultCommands(app *App) []Command {
	lookPath := exec.LookPath
	if app != nil && app.agentRunner != nil && app.agentRunner.LookPath != nil {
		lookPath = app.agentRunner.LookPath
	}
	availableProviders := agents.AvailableProviderKeys(lookPath)

	commands := []Command{
		{
			ID:           "refresh",
			Title:        "Refresh issues",
			Keywords:     []string{"refresh", "reload", "r"},
			ShortcutRune: 'r',
			Run: func(a *App) {
				go a.refreshIssues()
			},
		},
		{
			ID:              "search",
			Title:           "Search issues",
			Keywords:        []string{"search", "find", "s", "/"},
			ShortcutDisplay: "/", // Handled globally, not via ShortcutRune
			Run: func(a *App) {
				a.openSearchPalette()
			},
		},
		{
			ID:              "clear_search",
			Title:           "Clear search",
			Keywords:        []string{"clear", "reset"},
			ShortcutDisplay: "Esc", // Handled globally via Escape key
			Run: func(a *App) {
				a.setSearchQuery("")
			},
		},
		{
			ID:       "settings",
			Title:    "Settings",
			Keywords: []string{"settings", "config", "preferences"},
			Run: func(a *App) {
				a.ShowSettingsModal()
			},
		},
		{
			ID:       "edit_prompt_templates",
			Title:    "Edit agent prompt templates",
			Keywords: []string{"agent", "prompt", "prompts", "template", "templates"},
			Run: func(a *App) {
				a.ShowPromptTemplatesModal()
			},
		},
		{
			ID:       "sort_updated",
			Title:    "Sort by updated",
			Keywords: []string{"sort", "updated", "recent"},
			// No shortcut - ⌘+1/2/3 conflicts with terminal tab switching
			Run: func(a *App) {
				a.setSortField(SortByUpdatedAt)
			},
		},
		{
			ID:       "sort_created",
			Title:    "Sort by created",
			Keywords: []string{"sort", "created", "new"},
			// No shortcut - ⌘+1/2/3 conflicts with terminal tab switching
			Run: func(a *App) {
				a.setSortField(SortByCreatedAt)
			},
		},
		{
			ID:       "sort_priority",
			Title:    "Sort by priority",
			Keywords: []string{"sort", "priority", "urgent"},
			// No shortcut - ⌘+1/2/3 conflicts with terminal tab switching
			Run: func(a *App) {
				a.setSortField(SortByPriority)
			},
		},
		{
			ID:           "open_browser",
			Title:        "Open in browser",
			Keywords:     []string{"open", "browser", "o", "web"},
			ShortcutRune: 'o',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil || issue.URL == "" {
					return
				}
				_ = openURL(issue.URL)
			},
		},
		{
			ID:           "copy_id",
			Title:        "Copy issue ID",
			Keywords:     []string{"copy", "id", "c", "identifier"},
			ShortcutRune: 'y',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				_ = copyToClipboard(issue.Identifier)
			},
		},
		{
			ID:           "copy_url",
			Title:        "Copy issue URL",
			Keywords:     []string{"copy", "url", "link"},
			ShortcutRune: 'w', // 'w' for web URL
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil || issue.URL == "" {
					return
				}
				_ = copyToClipboard(issue.URL)
			},
		},
		{
			ID:       "ask_agent",
			Title:    "Ask agent about selected issue",
			Keywords: []string{"agent", "ai", "claude", "cursor", "assistant"},
			Run:      handleAskAgent,
		},
		{
			ID:           "assign_me",
			Title:        "Assign to me",
			Keywords:     []string{"assign", "me", "self", "take"},
			ShortcutRune: 'm',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				user := a.GetCurrentUser()
				if issue == nil || user == nil {
					return
				}
				go func() {
					ctx := context.Background()
					_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
						ID:         issue.ID,
						AssigneeID: &user.ID,
					})
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to assign issue issue=%s user=%s", issue.Identifier, user.DisplayName)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: assigned issue issue=%s user=%s", issue.Identifier, user.DisplayName)
						go a.refreshIssues(issue.ID)
					})
				}()
			},
		},
		{
			ID:           "unassign",
			Title:        "Unassign issue",
			Keywords:     []string{"unassign", "remove", "clear assignee"},
			ShortcutRune: 'u',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				emptyAssignee := ""
				go func() {
					ctx := context.Background()
					_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
						ID:         issue.ID,
						AssigneeID: &emptyAssignee,
					})
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to unassign issue issue=%s", issue.Identifier)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: unassigned issue issue=%s", issue.Identifier)
						go a.refreshIssues(issue.ID)
					})
				}()
			},
		},
		{
			ID:           "archive",
			Title:        "Archive issue",
			Keywords:     []string{"archive", "delete", "remove"},
			ShortcutRune: 'x',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				go func() {
					ctx := context.Background()
					err := a.GetAPI().ArchiveIssue(ctx, issue.ID)
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to archive issue issue=%s", issue.Identifier)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: archived issue issue=%s", issue.Identifier)
						// After archiving, the issue won't be in the list, so just refresh without ID
						go a.refreshIssues()
					})
				}()
			},
		},
		{
			ID:           "change_status",
			Title:        "Change status",
			Keywords:     []string{"status", "state", "workflow", "todo", "progress", "done"},
			ShortcutRune: 's',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.ShowStatusPicker(func(stateID string) {
					go func() {
						ctx := context.Background()
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:      issue.ID,
							StateID: &stateID,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to change status issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: changed status issue=%s", issue.Identifier)
							go a.refreshIssues(issue.ID)
						})
					}()
				})
			},
		},
		{
			ID:           "assign_user",
			Title:        "Assign to user",
			Keywords:     []string{"assign", "user", "team", "member"},
			ShortcutRune: 'a',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				// ShowUserPickerWithUnassign includes "Unassign" option at top
				a.ShowUserPickerWithUnassign(func(userID string) {
					go func() {
						ctx := context.Background()
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:         issue.ID,
							AssigneeID: &userID,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to assign issue to user issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: assigned/unassigned issue issue=%s", issue.Identifier)
							go a.refreshIssues(issue.ID)
						})
					}()
				})
			},
		},
		{
			ID:           "create_issue",
			Title:        "Create new issue",
			Keywords:     []string{"create", "new", "add", "issue"},
			ShortcutRune: 'n',
			Run: func(a *App) {
				teamID := a.GetSelectedTeamID()
				if teamID == "" {
					a.updateStatusBarWithError(fmt.Errorf("please select a team first"))
					return
				}
				a.ShowCreateIssueModal()
			},
		},
		{
			ID:           "edit_title",
			Title:        "Edit issue title",
			Keywords:     []string{"edit", "title", "rename"},
			ShortcutRune: 'T', // Changed to capital T to free 'e' for estimate
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.ShowEditTitleModal()
			},
		},
		{
			ID:           "edit_labels",
			Title:        "Edit issue labels",
			Keywords:     []string{"labels", "label", "tag", "tags"},
			ShortcutRune: 'f', // 'f' for flags/labels
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.ShowEditLabelsModal()
			},
		},
		{
			ID:           "git_checkout",
			Title:        "Checkout git branch",
			Keywords:     []string{"git", "checkout", "branch", "g"},
			ShortcutRune: 'g',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				workspace := strings.TrimSpace(a.config.AgentWorkspace)
				if workspace == "" {
					a.updateStatusBarWithError(fmt.Errorf("agent_workspace not configured in ~/.linear-tui/config.json"))
					return
				}
				branchName := strings.TrimSpace(issue.GitBranchName)
				if branchName == "" {
					branchName = slugifyBranchName(issue.Identifier, issue.Title)
				}
				go func() {
					err := checkoutGitBranch(branchName, workspace)
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: git checkout failed branch=%s workspace=%s", branchName, workspace)
							a.updateStatusBarWithError(fmt.Errorf("git checkout -b %s: %v", branchName, err))
							return
						}
						logger.Info("tui.commands: git checkout succeeded branch=%s", branchName)
						a.statusBar.SetText(fmt.Sprintf("%sBranch checked out: %s[-]", a.themeTags.Accent, branchName))
					})
				}()
			},
		},
		{
			ID:       "toggle_sub_issues",
			Title:    "Toggle sub-issues",
			Keywords: []string{"toggle", "expand", "collapse", "sub", "children"},
			// No shortcut - ⌘+T conflicts with new tab. Use Space key in table instead.
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.toggleIssueExpanded(issue.ID)
			},
		},
		{
			ID:           "view_parent",
			Title:        "View parent issue",
			Keywords:     []string{"parent", "up", "back"},
			ShortcutRune: 'P', // Changed to capital P to free 'p' for priority
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil || issue.Parent == nil {
					return
				}
				// Try to navigate to parent in the table
				parentRow := a.getRowForIssue(issue.Parent.ID)
				if parentRow > 0 {
					a.issuesTable.Select(parentRow, 0)
					if parent := a.getIssueFromRow(parentRow); parent != nil {
						a.onIssueSelected(*parent)
					}
				}
			},
		},
		{
			ID:           "expand_all",
			Title:        "Expand all sub-issues",
			Keywords:     []string{"expand", "all", "open"},
			ShortcutRune: ']',
			Run: func(a *App) {
				a.issuesMu.RLock()
				issues := a.issues
				a.issuesMu.RUnlock()
				ExpandAll(a.expandedState, issues)
				// Rebuild rows for both sections
				currentUserID := ""
				if a.currentUser != nil {
					currentUserID = a.currentUser.ID
				}
				myIssues, otherIssues := splitIssuesByAssignee(issues, currentUserID)
				a.myIssueRows, a.myIDToIssue = BuildIssueRows(myIssues, a.expandedState)
				a.otherIssueRows, a.otherIDToIssue = BuildIssueRows(otherIssues, a.expandedState)

				// Legacy: keep old fields for backward compatibility
				a.issueRows = make([]IssueRow, 0, len(a.myIssueRows)+len(a.otherIssueRows))
				a.issueRows = append(a.issueRows, a.myIssueRows...)
				a.issueRows = append(a.issueRows, a.otherIssueRows...)
				a.idToIssue = make(map[string]*linearapi.Issue)
				for k, v := range a.myIDToIssue {
					a.idToIssue[k] = v
				}
				for k, v := range a.otherIDToIssue {
					a.idToIssue[k] = v
				}

				// Update layout
				a.updateIssuesColumnLayout()

				// Render both tables, preserving selection
				var selectedMyIssueID, selectedOtherIssueID string
				a.issuesMu.RLock()
				selectedIssue := a.selectedIssue
				a.issuesMu.RUnlock()
				if selectedIssue != nil {
					if _, ok := a.myIDToIssue[selectedIssue.ID]; ok {
						selectedMyIssueID = selectedIssue.ID
						a.activeIssuesSection = IssuesSectionMy
					} else if _, ok := a.otherIDToIssue[selectedIssue.ID]; ok {
						selectedOtherIssueID = selectedIssue.ID
						a.activeIssuesSection = IssuesSectionOther
					}
				}

				renderIssuesTableModel(a.myIssuesTable, a.myIssueRows, a.myIDToIssue, selectedMyIssueID, a.theme)
				renderIssuesTableModel(a.otherIssuesTable, a.otherIssueRows, a.otherIDToIssue, selectedOtherIssueID, a.theme)
			},
		},
		{
			ID:           "collapse_all",
			Title:        "Collapse all sub-issues",
			Keywords:     []string{"collapse", "all", "close"},
			ShortcutRune: '[',
			Run: func(a *App) {
				CollapseAll(a.expandedState)
				// Rebuild rows for both sections
				currentUserID := ""
				if a.currentUser != nil {
					currentUserID = a.currentUser.ID
				}
				a.issuesMu.RLock()
				issues := a.issues
				a.issuesMu.RUnlock()
				myIssues, otherIssues := splitIssuesByAssignee(issues, currentUserID)
				a.myIssueRows, a.myIDToIssue = BuildIssueRows(myIssues, a.expandedState)
				a.otherIssueRows, a.otherIDToIssue = BuildIssueRows(otherIssues, a.expandedState)

				// Legacy: keep old fields for backward compatibility
				a.issueRows = make([]IssueRow, 0, len(a.myIssueRows)+len(a.otherIssueRows))
				a.issueRows = append(a.issueRows, a.myIssueRows...)
				a.issueRows = append(a.issueRows, a.otherIssueRows...)
				a.idToIssue = make(map[string]*linearapi.Issue)
				for k, v := range a.myIDToIssue {
					a.idToIssue[k] = v
				}
				for k, v := range a.otherIDToIssue {
					a.idToIssue[k] = v
				}

				// Update layout
				a.updateIssuesColumnLayout()

				// Render both tables, preserving selection
				var selectedMyIssueID, selectedOtherIssueID string
				a.issuesMu.RLock()
				selectedIssue := a.selectedIssue
				a.issuesMu.RUnlock()
				if selectedIssue != nil {
					if _, ok := a.myIDToIssue[selectedIssue.ID]; ok {
						selectedMyIssueID = selectedIssue.ID
						a.activeIssuesSection = IssuesSectionMy
					} else if _, ok := a.otherIDToIssue[selectedIssue.ID]; ok {
						selectedOtherIssueID = selectedIssue.ID
						a.activeIssuesSection = IssuesSectionOther
					}
				}

				renderIssuesTableModel(a.myIssuesTable, a.myIssueRows, a.myIDToIssue, selectedMyIssueID, a.theme)
				renderIssuesTableModel(a.otherIssuesTable, a.otherIssueRows, a.otherIDToIssue, selectedOtherIssueID, a.theme)
			},
		},
		{
			ID:           "create_sub_issue",
			Title:        "Create sub-issue",
			Keywords:     []string{"create", "sub", "child", "new"},
			ShortcutRune: 'b',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				// Create sub-issue with current issue as parent
				a.ShowCreateSubIssueModal(issue.ID)
			},
		},
		{
			ID:           "set_parent",
			Title:        "Set parent issue",
			Keywords:     []string{"set", "parent", "link"},
			ShortcutRune: 'i',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				// Cannot set parent if this issue has children
				if len(issue.Children) > 0 {
					logger.Warning("tui.commands: cannot set parent on issue with sub-issues issue=%s", issue.Identifier)
					return
				}
				a.ShowParentIssuePicker(func(parentID string) {
					go func() {
						ctx := context.Background()
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:       issue.ID,
							ParentID: &parentID,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to set parent issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: set parent issue=%s", issue.Identifier)
							go a.refreshIssues(issue.ID)
						})
					}()
				})
			},
		},
		{
			ID:           "remove_parent",
			Title:        "Remove parent",
			Keywords:     []string{"remove", "parent", "unlink", "top"},
			ShortcutRune: 'D', // Changed to capital D to free 'd' for due date
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil || issue.Parent == nil {
					return
				}
				emptyParent := ""
				go func() {
					ctx := context.Background()
					_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
						ID:       issue.ID,
						ParentID: &emptyParent,
					})
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to remove parent issue=%s", issue.Identifier)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: removed parent issue=%s", issue.Identifier)
						go a.refreshIssues(issue.ID)
					})
				}()
			},
		},
		{
			ID:           "add_comment",
			Title:        "Add comment",
			Keywords:     []string{"add", "comment", "reply", "t"},
			ShortcutRune: 't',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.createCommentModal.Show(issue.ID, a.handleCreateComment)
			},
		},
		{
			ID:       "start_cycle",
			Title:    "Start Cycle",
			Keywords: []string{"cycle", "start", "sprint"},
			Run: func(a *App) {
				cyc := a.selectedCycle
				if cyc == nil {
					a.updateStatusBarWithError(fmt.Errorf("no cycle selected"))
					return
				}
				cycleID := cyc.ID
				teamID := ""
				if a.selectedNavigation != nil {
					teamID = a.selectedNavigation.TeamID
				}
				go func() {
					ctx := context.Background()
					err := a.GetAPI().StartCycle(ctx, cycleID)
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to start cycle cycle_id=%s", cycleID)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: started cycle cycle_id=%s", cycleID)
						// Invalidate cycles cache so the sidebar refreshes on next expand
						if teamID != "" {
							a.cache.InvalidateCycles(teamID)
						}
						a.updateStatusBar()
					})
				}()
			},
		},
		{
			ID:       "add_to_cycle",
			Title:    "Add Issue to Cycle",
			Keywords: []string{"cycle", "add", "sprint", "assign cycle"},
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					a.updateStatusBarWithError(fmt.Errorf("no issue selected"))
					return
				}
				issueID := issue.ID
				issueIdentifier := issue.Identifier
				a.ShowCyclePicker(func(cycleID string) {
					go func() {
						ctx := context.Background()
						err := a.GetAPI().AddIssueToCycle(ctx, issueID, cycleID)
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to add issue to cycle issue=%s cycle_id=%s", issueIdentifier, cycleID)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: added issue to cycle issue=%s cycle_id=%s", issueIdentifier, cycleID)
							go a.refreshIssues(issueID)
						})
					}()
				})
			},
		},
		// Feature 1: Priority change
		{
			ID:           "change_priority",
			Title:        "Change issue priority",
			Keywords:     []string{"priority", "urgent", "high", "medium", "low"},
			ShortcutRune: 'p',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.ShowPriorityPicker(func(priority int) {
					go func() {
						ctx := context.Background()
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:       issue.ID,
							Priority: &priority,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to change priority issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: changed priority issue=%s priority=%d", issue.Identifier, priority)
							go a.refreshIssues(issue.ID)
						})
					}()
				})
			},
		},
		// Feature 3: Estimate set
		{
			ID:           "set_estimate",
			Title:        "Set story point estimate",
			Keywords:     []string{"estimate", "points", "story", "size"},
			ShortcutRune: 'e',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				a.ShowEstimatePicker(func(estimate *float64) {
					go func() {
						ctx := context.Background()
						// When estimate is nil (Clear), set to 0 to clear
						var updateEstimate *float64
						if estimate != nil {
							updateEstimate = estimate
						} else {
							zero := 0.0
							updateEstimate = &zero
						}
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:       issue.ID,
							Estimate: updateEstimate,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to set estimate issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: set estimate issue=%s", issue.Identifier)
							go a.refreshIssues(issue.ID)
						})
					}()
				})
			},
		},
		// Feature 4: Due date set
		{
			ID:           "set_due_date",
			Title:        "Set due date",
			Keywords:     []string{"due", "date", "deadline", "duedate"},
			ShortcutRune: 'd',
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					return
				}
				currentDate := issue.DueDate
				a.ShowDueDateModal(issue.ID, currentDate, func(issueID, date string) {
					go func() {
						ctx := context.Background()
						// "clear" or empty string = clear due date
						var dueDate string
						if strings.ToLower(strings.TrimSpace(date)) == "clear" {
							dueDate = ""
						} else {
							dueDate = strings.TrimSpace(date)
						}
						_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
							ID:      issueID,
							DueDate: &dueDate,
						})
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to set due date issue=%s", issueID)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: set due date issue=%s date=%s", issueID, dueDate)
							go a.refreshIssues(issueID)
						})
					}()
				})
			},
		},
		// Feature 6: Create relation
		{
			ID:       "create_relation",
			Title:    "Create issue relation",
			Keywords: []string{"relation", "link", "blocks", "duplicate", "related"},
			Run: func(a *App) {
				issue := a.GetSelectedIssue()
				if issue == nil {
					a.updateStatusBarWithError(fmt.Errorf("no issue selected"))
					return
				}
				a.ShowRelationModal(issue.ID, func(issueID, relatedIssueID, relationType string) {
					go func() {
						ctx := context.Background()
						err := a.GetAPI().CreateIssueRelation(ctx, issueID, relatedIssueID, relationType)
						a.QueueUpdateDraw(func() {
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: failed to create relation issue=%s", issue.Identifier)
								a.updateStatusBarWithError(err)
								return
							}
							logger.Info("tui.commands: created relation issue=%s type=%s", issue.Identifier, relationType)
							go a.refreshIssues(issueID)
						})
					}()
				})
			},
		},
		// Feature 7: Bulk status change
		{
			ID:       "bulk_status_change",
			Title:    "Bulk: change status for selected issues",
			Keywords: []string{"bulk", "status", "multiple"},
			Run: func(a *App) {
				if len(a.selectedIssueIDs) == 0 {
					a.updateStatusBarWithError(fmt.Errorf("no issues selected (use Space to select)"))
					return
				}
				issueIDs := make([]string, 0, len(a.selectedIssueIDs))
				for id := range a.selectedIssueIDs {
					issueIDs = append(issueIDs, id)
				}
				a.ShowStatusPicker(func(stateID string) {
					go func() {
						ctx := context.Background()
						for _, issueID := range issueIDs {
							id := issueID
							_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
								ID:      id,
								StateID: &stateID,
							})
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: bulk status change failed issue_id=%s", id)
							}
						}
						a.QueueUpdateDraw(func() {
							logger.Info("tui.commands: bulk status change applied count=%d", len(issueIDs))
							a.ClearBulkSelect()
							go a.refreshIssues()
						})
					}()
				})
			},
		},
		// Feature 7: Bulk add to cycle
		{
			ID:       "bulk_add_to_cycle",
			Title:    "Bulk: add selected issues to cycle",
			Keywords: []string{"bulk", "cycle", "sprint", "multiple"},
			Run: func(a *App) {
				if len(a.selectedIssueIDs) == 0 {
					a.updateStatusBarWithError(fmt.Errorf("no issues selected (use Space to select)"))
					return
				}
				issueIDs := make([]string, 0, len(a.selectedIssueIDs))
				for id := range a.selectedIssueIDs {
					issueIDs = append(issueIDs, id)
				}
				a.ShowCyclePicker(func(cycleID string) {
					go func() {
						ctx := context.Background()
						for _, issueID := range issueIDs {
							err := a.GetAPI().AddIssueToCycle(ctx, issueID, cycleID)
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: bulk add to cycle failed issue_id=%s cycle_id=%s", issueID, cycleID)
							}
						}
						a.QueueUpdateDraw(func() {
							logger.Info("tui.commands: bulk add to cycle applied count=%d cycle_id=%s", len(issueIDs), cycleID)
							a.ClearBulkSelect()
							go a.refreshIssues()
						})
					}()
				})
			},
		},
		// Feature 7: Bulk assign
		{
			ID:       "bulk_assign",
			Title:    "Bulk: assign selected issues to user",
			Keywords: []string{"bulk", "assign", "multiple"},
			Run: func(a *App) {
				if len(a.selectedIssueIDs) == 0 {
					a.updateStatusBarWithError(fmt.Errorf("no issues selected (use Space to select)"))
					return
				}
				issueIDs := make([]string, 0, len(a.selectedIssueIDs))
				for id := range a.selectedIssueIDs {
					issueIDs = append(issueIDs, id)
				}
				a.ShowUserPickerWithUnassign(func(userID string) {
					go func() {
						ctx := context.Background()
						for _, issueID := range issueIDs {
							id := issueID
							_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
								ID:         id,
								AssigneeID: &userID,
							})
							if err != nil {
								logger.ErrorWithErr(err, "tui.commands: bulk assign failed issue_id=%s", id)
							}
						}
						a.QueueUpdateDraw(func() {
							logger.Info("tui.commands: bulk assign applied count=%d", len(issueIDs))
							a.ClearBulkSelect()
							go a.refreshIssues()
						})
					}()
				})
			},
		},
		// Feature 8: Mark notification as read
		{
			ID:       "mark_read",
			Title:    "Mark notification as read",
			Keywords: []string{"mark", "read", "notification"},
			Run: func(a *App) {
				if !a.inNotificationsView || len(a.notifications) == 0 {
					return
				}
				// Get selected row from otherIssuesTable (used for notifications)
				row, _ := a.otherIssuesTable.GetSelection()
				notifIdx := row - 1 // Subtract header row
				if notifIdx < 0 || notifIdx >= len(a.notifications) {
					return
				}
				notif := a.notifications[notifIdx]
				if notif.ReadAt != "" {
					return // Already read
				}
				go func() {
					ctx := context.Background()
					err := a.GetAPI().MarkNotificationRead(ctx, notif.ID)
					a.QueueUpdateDraw(func() {
						if err != nil {
							logger.ErrorWithErr(err, "tui.commands: failed to mark notification read notification_id=%s", notif.ID)
							a.updateStatusBarWithError(err)
							return
						}
						logger.Info("tui.commands: marked notification read notification_id=%s", notif.ID)
						// Refresh notifications
						a.LoadAndShowNotifications()
					})
				}()
			},
		},
	}
	if len(availableProviders) == 0 {
		filtered := make([]Command, 0, len(commands))
		for _, command := range commands {
			if command.ID == "ask_agent" {
				continue
			}
			filtered = append(filtered, command)
		}
		commands = filtered
	}
	return commands
}

// openURL opens a URL in the default browser.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		logger.Warning("tui.commands: unsupported OS for opening URLs os=%s", runtime.GOOS)
		return nil
	}

	if err := cmd.Start(); err != nil {
		logger.ErrorWithErr(err, "tui.commands: failed to open URL url=%s", url)
		return err
	}

	logger.Debug("tui.commands: opened URL in browser url=%s", url)
	return nil
}

// checkoutGitBranch runs `git checkout -b <branchName>` in workspace.
func checkoutGitBranch(branchName, workspace string) error {
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = workspace
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// slugifyBranchName produces a safe git branch name from an issue identifier and title.
// Format: <identifier>-<slug> where slug replaces non-alphanumeric chars with '-'
// and the whole result is truncated to 50 characters.
func slugifyBranchName(identifier, title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(identifier + "-" + title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := b.String()
	// Collapse multiple consecutive dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	// Trim leading/trailing dashes
	result = strings.Trim(result, "-")
	if len(result) > 50 {
		result = result[:50]
		result = strings.TrimRight(result, "-")
	}
	return result
}

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("clip")
	default:
		logger.Warning("tui.commands: unsupported OS for clipboard operations os=%s", runtime.GOOS)
		return nil
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.ErrorWithErr(err, "tui.commands: failed to get stdin pipe for clipboard command")
		return err
	}

	if err := cmd.Start(); err != nil {
		logger.ErrorWithErr(err, "tui.commands: failed to start clipboard command")
		return err
	}

	_, err = stdin.Write([]byte(text))
	if err != nil {
		logger.ErrorWithErr(err, "tui.commands: failed to write to clipboard")
		return err
	}
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		logger.ErrorWithErr(err, "tui.commands: clipboard command failed")
		return err
	}

	logger.Debug("tui.commands: copied to clipboard text_length=%d", len(text))
	return nil
}

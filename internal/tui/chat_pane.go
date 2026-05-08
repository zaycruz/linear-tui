package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ChatPane is the bottom-panel natural language interface.
type ChatPane struct {
	root    *tview.Flex
	history *tview.TextView
	input   *tview.InputField
	buf     strings.Builder
	Busy    bool
}

func newChatPane(onSubmit func(string), onClose func()) *ChatPane {
	history := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)
	history.SetBorder(false)
	history.SetBackgroundColor(tcell.ColorDefault)

	input := tview.NewInputField().
		SetLabel("> ").
		SetPlaceholder("ask about your issues...").
		SetPlaceholderTextColor(tcell.ColorDarkGray).
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetLabelColor(tcell.ColorGreen)
	input.SetBorder(false)
	input.SetBackgroundColor(tcell.ColorDefault)

	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			onClose()
			return nil
		}
		return event
	})

	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		text := strings.TrimSpace(input.GetText())
		if text == "" {
			return
		}
		input.SetText("")
		onSubmit(text)
	})

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(history, 0, 1, false).
		AddItem(input, 1, 0, true)
	root.SetBorder(true).
		SetTitle(" Chat [? to toggle, Esc to close] ").
		SetBorderColor(tcell.ColorDarkSlateGray)
	root.SetBackgroundColor(tcell.ColorDefault)

	return &ChatPane{root: root, history: history, input: input}
}

func (c *ChatPane) addUser(text string) {
	if c.buf.Len() > 0 {
		c.buf.WriteString("\n")
	}
	c.buf.WriteString("[yellow]You:[white] ")
	c.buf.WriteString(tview.Escape(text))
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
}

func (c *ChatPane) addAssistant(text string) {
	if c.buf.Len() > 0 {
		c.buf.WriteString("\n")
	}
	c.buf.WriteString("[cyan]AI:[white]  ")
	c.buf.WriteString(tview.Escape(text))
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
	c.Busy = false
}

// startStream writes the "AI: " header for a streaming response.
// Call this before the first token arrives.
func (c *ChatPane) startStream() {
	if c.buf.Len() > 0 {
		c.buf.WriteString("\n")
	}
	c.buf.WriteString("[cyan]AI:[white]  ")
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
}

// appendToken appends a single streamed token and refreshes the view.
func (c *ChatPane) appendToken(token string) {
	c.buf.WriteString(tview.Escape(token))
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
}

// finalizeStream marks the response complete after streaming finishes.
func (c *ChatPane) finalizeStream() {
	c.Busy = false
}

func (c *ChatPane) addError(msg string) {
	if c.buf.Len() > 0 {
		c.buf.WriteString("\n")
	}
	c.buf.WriteString("[red]Error:[white] ")
	c.buf.WriteString(tview.Escape(msg))
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
	c.Busy = false
}

func (c *ChatPane) setStatus(msg string) {
	if c.buf.Len() > 0 {
		c.buf.WriteString("\n")
	}
	c.buf.WriteString("[darkgray]" + tview.Escape(msg) + "[-]")
	c.history.SetText(c.buf.String())
	c.history.ScrollToEnd()
}

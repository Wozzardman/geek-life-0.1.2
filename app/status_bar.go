package main

import (
	"sync/atomic"
	"time"

	"github.com/rivo/tview"
)

// StatusBar displays hints and messages at the bottom of app
type StatusBar struct {
	*tview.Pages
	message   *tview.TextView
	container *tview.Application
}

// Name of page keys
const (
	defaultPage = "default"
	messagePage = "message"
)

// Used to skip queued restore of statusBar
// in case of new showForSeconds within waiting period
var statusGeneration uint64

func prepareStatusBar(app *tview.Application) *StatusBar {
	statusBar = &StatusBar{
		Pages:     tview.NewPages(),
		message:   tview.NewTextView().SetDynamicColors(true).SetText("Loading..."),
		container: app,
	}

	statusBar.AddPage(messagePage, statusBar.message, true, true)
	statusBar.AddPage(defaultPage,
		tview.NewGrid(). // Content will not be modified, So, no need to declare explicitly
					SetColumns(0, 0, 0, 0).
					SetRows(0).
					AddItem(tview.NewTextView().SetText(": / Ctrl+P Commands"), 0, 0, 1, 1, 0, 0, false).
					AddItem(tview.NewTextView().SetText("/ Search").SetTextAlign(tview.AlignCenter), 0, 1, 1, 1, 0, 0, false).
					AddItem(tview.NewTextView().SetText("? Keyboard Help").SetTextAlign(tview.AlignCenter), 0, 2, 1, 1, 0, 0, false).
					AddItem(tview.NewTextView().SetText("j/k Move   Enter Open   Esc Back").SetTextAlign(tview.AlignRight), 0, 3, 1, 1, 0, 0, false),
		true,
		true,
	)

	return statusBar
}

func (bar *StatusBar) restore() {
	bar.container.QueueUpdateDraw(func() {
		bar.SwitchToPage(defaultPage)
	})
}

func (bar *StatusBar) showForSeconds(message string, timeout int) {
	if bar.container == nil {
		return
	}

	bar.message.SetText(message)
	bar.SwitchToPage(messagePage)
	generation := atomic.AddUint64(&statusGeneration, 1)

	go func() {
		time.Sleep(time.Second * time.Duration(timeout))
		if atomic.LoadUint64(&statusGeneration) == generation {
			bar.restore()
		}
	}()
}

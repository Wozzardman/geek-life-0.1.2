package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const activeOverlayPage = "overlay"

var overlayReturnFocus tview.Primitive

func centeredPrimitive(width, height int, primitive tview.Primitive) tview.Primitive {
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(primitive, width, 0, true).
			AddItem(nil, 0, 1, false), height, 0, true).
		AddItem(nil, 0, 1, false)
}

func showOverlay(primitive tview.Primitive, focus tview.Primitive, width, height int) {
	if rootPages == nil {
		return
	}
	overlayReturnFocus = app.GetFocus()
	rootPages.RemovePage(activeOverlayPage)
	rootPages.AddPage(activeOverlayPage, centeredPrimitive(width, height, primitive), true, true)
	app.SetFocus(focus)
}

func showFullOverlay(primitive tview.Primitive, focus tview.Primitive) {
	if rootPages == nil {
		return
	}
	overlayReturnFocus = app.GetFocus()
	rootPages.RemovePage(activeOverlayPage)
	rootPages.AddPage(activeOverlayPage, primitive, true, true)
	app.SetFocus(focus)
}

func closeOverlay() {
	if rootPages == nil {
		return
	}
	rootPages.RemovePage(activeOverlayPage)
	if overlayReturnFocus != nil {
		app.SetFocus(overlayReturnFocus)
	}
}

func showConfirm(title, message string, confirmed func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Cancel", "Confirm"})
	modal.SetTitle(" " + title + " ").SetBorder(true)
	modal.SetDoneFunc(func(buttonIndex int, _ string) {
		closeOverlay()
		if buttonIndex == 1 && confirmed != nil {
			confirmed()
		}
	})
	showOverlay(modal, modal, 68, 13)
}

func showTextPrompt(title, label, initial string, submitted func(string)) {
	input := makeLightTextInput("").SetLabel(label).SetText(initial)
	hint := tview.NewTextView().SetText(" Enter = save    Esc = cancel").SetTextColor(tcell.ColorDimGray)
	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(input, 1, 0, true).
		AddItem(nil, 1, 0, false).
		AddItem(hint, 1, 0, false)
	panel.SetBorder(true).SetTitle(" " + title + " ")
	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			value := input.GetText()
			closeOverlay()
			if submitted != nil {
				submitted(value)
			}
		case tcell.KeyEsc:
			closeOverlay()
		}
	})
	showOverlay(panel, input, 72, 6)
}

func showKeyboardHelp() {
	help := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	help.SetBorder(true).SetTitle(" Keyboard Help — Esc/q to close ")
	help.SetText(`[lime::b]GLOBAL[::-]
  : / Ctrl+P   command palette          /   structured task search
  ?            this keyboard help       p   focus Projects
  t            focus Tasks              Ctrl+N   quick new task

[lime::b]NAVIGATION[::-]
  j / Down     move down                 k / Up   move up
  Enter        open/select               Esc / h  move back/left
  Tab          next item                 Shift+Tab previous item

[lime::b]TASK LIST[::-]
  n            quick-add task            Space    complete/resume
  !            cycle priority            s        pin/unpin
  [ / ]        reorder in project        D        move to Trash
  u            undo last delete          /        search all tasks

Quick-add metadata can be typed with the title:
  Ship release #work !3 @tomorrow ~weekly +pin
  #tag   !1/!2/!3   @today/@tomorrow/@YYYY-MM-DD
  ~daily/~weekly/~monthly/~yearly   +pin

[lime::b]TASK DETAIL[::-]
  e            edit note                 v        external editor
  i / Ctrl+V   paste clipboard image     [ / ]    previous/next image
  o            REAL PIXELS (auto)        O        compatibility cell viewer
  d            edit due date             g        edit tags
  !            cycle priority            s        pin/unpin
  R            cycle recurrence          r        rename
  Space        complete/resume            D        move to Trash
  x            export task Markdown

[lime::b]IMAGE VIEWER[::-]
  n            REAL PIXELS (auto)
  b            sextant/color mode        +/- or 1-9   zoom
  h/j/k/l      pan (arrows also work)    f            fit image
  [ / ]        previous/next image       Esc/q        close

[lime::b]STRUCTURED SEARCH[::-]
  plain words search title, notes, tags, and project
  #work        tag                       @project  project name
  !2           medium-or-higher          is:done / is:open / is:pinned
  due:today    due today                 due:overdue / due:none

[lime::b]PROJECTS[::-]
  n            new project               r        rename project
  a            archive/unarchive         c        move completed to Trash
  d            delete with confirmation`)
	help.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, column := help.GetScrollOffset()
		switch event.Key() {
		case tcell.KeyEsc:
			closeOverlay()
			return nil
		case tcell.KeyDown:
			help.ScrollTo(row+1, column)
			return nil
		case tcell.KeyUp:
			if row > 0 {
				help.ScrollTo(row-1, column)
			}
			return nil
		}
		switch event.Rune() {
		case 'q', '?':
			closeOverlay()
			return nil
		case 'j':
			help.ScrollTo(row+1, column)
			return nil
		case 'k':
			if row > 0 {
				help.ScrollTo(row-1, column)
			}
			return nil
		}
		return event
	})
	showOverlay(help, help, 86, 28)
}

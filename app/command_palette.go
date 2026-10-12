package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type paletteCommand struct {
	name     string
	shortcut string
	keywords string
	action   func()
}

type CommandPalette struct {
	*tview.Flex
	input    *tview.InputField
	list     *tview.List
	commands []paletteCommand
	visible  []paletteCommand
}

func NewCommandPalette() *CommandPalette {
	palette := &CommandPalette{
		Flex:  tview.NewFlex().SetDirection(tview.FlexRow),
		input: makeLightTextInput("Type a command"),
		list:  tview.NewList().ShowSecondaryText(true),
	}
	palette.SetBorder(true).SetTitle(" Command Palette ")
	palette.list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	palette.
		AddItem(palette.input, 1, 0, true).
		AddItem(palette.list, 0, 1, false)
	palette.commands = []paletteCommand{
		{name: "Search all tasks", shortcut: "/", keywords: "find filter query", action: taskPane.ShowSearch},
		{name: "New task in current project", shortcut: "Ctrl+N", keywords: "add capture", action: func() { taskPane.StartNewTask() }},
		{name: "New project", shortcut: "p n", keywords: "add list", action: func() { app.SetFocus(projectPane.newProject) }},
		{name: "Rename current project", shortcut: "r", keywords: "edit title", action: projectPane.RenameActiveProject},
		{name: "View: All open", keywords: "smart list", action: func() { taskPane.LoadDynamicList("all") }},
		{name: "View: Today", keywords: "smart list due", action: func() { taskPane.LoadDynamicList("today") }},
		{name: "View: Overdue", keywords: "smart list due late", action: func() { taskPane.LoadDynamicList("overdue") }},
		{name: "View: Tomorrow", keywords: "smart list due", action: func() { taskPane.LoadDynamicList("tomorrow") }},
		{name: "View: Upcoming 7 days", keywords: "smart list due week", action: func() { taskPane.LoadDynamicList("upcoming") }},
		{name: "View: Pinned", keywords: "smart list star", action: func() { taskPane.LoadDynamicList("pinned") }},
		{name: "View: High priority", keywords: "smart list urgent", action: func() { taskPane.LoadDynamicList("high") }},
		{name: "View: Completed", keywords: "smart list done", action: func() { taskPane.LoadDynamicList("completed") }},
		{name: "View: Unscheduled", keywords: "smart list no due", action: func() { taskPane.LoadDynamicList("unscheduled") }},
		{name: "View: Trash", keywords: "smart list deleted restore", action: func() { taskPane.LoadDynamicList("trash") }},
		{name: "Open task image in real pixels", shortcut: "o", keywords: "image picture sixel kitty iterm", action: taskDetailPane.openBestImageViewer},
		{name: "Open task image in compatibility mode", shortcut: "O", keywords: "image picture cell block portable", action: taskDetailPane.openImageViewer},
		{name: "Focus projects", shortcut: "p", keywords: "navigate", action: func() { app.SetFocus(projectPane) }},
		{name: "Focus tasks", shortcut: "t", keywords: "navigate", action: func() { app.SetFocus(taskPane.list) }},
		{name: "Create JSON backup", keywords: "export safety data", action: backupNow},
		{name: "Keyboard help", shortcut: "?", keywords: "shortcuts keys", action: showKeyboardHelp},
		{name: "Quit Geek-life", shortcut: "Ctrl+C", keywords: "exit", action: app.Stop},
	}
	palette.input.SetChangedFunc(func(query string) { palette.filter(query) })
	palette.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			closeOverlay()
			return nil
		case tcell.KeyEnter:
			palette.executeCurrent()
			return nil
		case tcell.KeyDown, tcell.KeyCtrlN:
			palette.list.SetCurrentItem(palette.list.GetCurrentItem() + 1)
			return nil
		case tcell.KeyUp, tcell.KeyCtrlP:
			palette.list.SetCurrentItem(palette.list.GetCurrentItem() - 1)
			return nil
		}
		return event
	})
	palette.filter("")
	return palette
}

func (palette *CommandPalette) Show() {
	palette.input.SetText("")
	palette.filter("")
	showOverlay(palette, palette.input, 72, 20)
}

func (palette *CommandPalette) filter(query string) {
	palette.visible = palette.visible[:0]
	palette.list.Clear()
	for _, command := range palette.commands {
		haystack := command.name + " " + command.keywords + " " + command.shortcut
		if !fuzzyMatch(haystack, query) {
			continue
		}
		palette.visible = append(palette.visible, command)
		idx := len(palette.visible) - 1
		palette.list.AddItem(command.name, command.shortcut, 0, func(index int) func() {
			return func() {
				palette.list.SetCurrentItem(index)
				palette.executeCurrent()
			}
		}(idx))
	}
}

func (palette *CommandPalette) executeCurrent() {
	idx := palette.list.GetCurrentItem()
	if idx < 0 || idx >= len(palette.visible) {
		return
	}
	command := palette.visible[idx]
	closeOverlay()
	if command.action != nil {
		command.action()
	}
}

func fuzzyMatch(value, query string) bool {
	value = strings.ToLower(value)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	position := 0
	for _, wanted := range query {
		found := false
		for position < len(value) {
			current := rune(value[position])
			position++
			if current == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

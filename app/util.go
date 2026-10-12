package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
)

var blankCell = tview.NewTextView()

func makeHorizontalLine(lineChar rune, color tcell.Color) *tview.TextView {
	hr := tview.NewTextView()
	hr.SetDrawFunc(func(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
		// Draw a horizontal line across the middle of the box.
		style := tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack)
		centerY := y + height/2
		for cx := x; cx < x+width; cx++ {
			screen.SetContent(cx, centerY, lineChar, nil, style)
		}

		// Space for other content.
		return x + 1, centerY + 1, width - 2, height - (centerY + 1 - y)
	})

	return hr
}

func makeLightTextInput(placeholder string) *tview.InputField {
	return tview.NewInputField().
		SetPlaceholder(placeholder).
		SetPlaceholderTextColor(tcell.ColorDarkSlateBlue).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldBackgroundColor(tcell.ColorLightBlue)
}

// parseDateInput accepts the same terse dates as quick task creation.
func parseDateInput(inputText string, now time.Time) (time.Time, bool) {
	value := strings.ToLower(strings.TrimSpace(inputText))
	if date, ok := parseQuickDueDate(value, now); ok {
		return date, true
	}
	return time.Time{}, false
}

// If input text is a valid date, parse it. Or get current date.
func parseDateInputOrCurrent(inputText string) time.Time {
	if dateTime, ok := parseDateInput(inputText, time.Now()); ok {
		return dateTime
	}

	return toDate(time.Now())
}

func toDate(dateTime time.Time) time.Time {
	local := dateTime.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func makeButton(label string, handler func()) *tview.Button {
	btn := tview.NewButton(label).SetSelectedFunc(handler).
		SetLabelColor(tcell.ColorWhite)

	btn.SetBackgroundColor(tcell.ColorCornflowerBlue)

	return btn
}

func ignoreKeyEvt() bool {
	switch focused := app.GetFocus().(type) {
	case *tview.InputField:
		return true
	case *femto.View:
		return !focused.Readonly
	default:
		return false
	}
}

func removeThirdCol() {
	contents.RemoveItem(taskDetailPane)
	contents.RemoveItem(projectDetailPane)
}

func getTaskTitleColor(task model.Task) string {
	colorName := "white"

	if task.DeletedAt != 0 {
		colorName = "gray"
	} else if task.Completed {
		colorName = "darkgreen"
	} else if task.DueDate != 0 {
		due := toDate(time.Unix(task.DueDate, 0))
		today := toDate(time.Now())
		if due.Equal(today) {
			colorName = "orange"
		} else if due.Before(today) {
			colorName = "red"
		}
	}

	return colorName
}

func makeTaskListingTitle(task model.Task) string {
	checkbox := tview.Escape("[ ]")
	if task.Completed {
		checkbox = tview.Escape("[x]")
	}

	pin := ""
	if task.Pinned {
		pin = "[deepskyblue]★[white] "
	}
	priority := ""
	switch task.Priority {
	case 1:
		priority = "[yellow]·[white] "
	case 2:
		priority = "[orange]![white] "
	case 3:
		priority = "[red]!![white] "
	}
	prefix := ""
	if projectPane.GetActiveProject() == nil {
		if project, err := projectRepo.GetByID(task.ProjectID); err == nil {
			prefix = "[::d]" + tview.Escape(project.Title) + ":[::-] "
		}
	}
	tags := ""
	if len(task.Tags) != 0 {
		visibleTags := task.Tags
		if len(visibleTags) > 2 {
			visibleTags = visibleTags[:2]
		}
		tags = " [steelblue]#" + tview.Escape(strings.Join(visibleTags, " #"))
	}
	due := ""
	if task.DueDate != 0 && !task.Completed {
		dueTime := time.Unix(task.DueDate, 0)
		due = " [::d]" + dueTime.Format("Jan 02")
	}

	return fmt.Sprintf("[%s]%s%s %s%s%s%s", getTaskTitleColor(task), pin, priority, checkbox, prefix, tview.Escape(task.Title), tags+due)
}

func findProjectByID(id int64) *model.Project {
	for i := range projectPane.projects {
		if projectPane.projects[i].ID == id {
			return &projectPane.projects[i]
		}
	}
	for i := range projectPane.archivedProjects {
		if projectPane.archivedProjects[i].ID == id {
			return &projectPane.archivedProjects[i]
		}
	}

	return nil
}

package main

import (
	goruntime "runtime"

	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/pgavlin/femto/runtime"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
	"github.com/ajaxray/geek-life/util"
)

const dateLayoutISO = "2006-01-02"
const dateLayoutHuman = "02 Jan, Monday"

// TaskDetailPane displays detailed info of a Task
type TaskDetailPane struct {
	*tview.Flex
	header           *TaskDetailHeader
	taskDateDisplay  *tview.TextView
	taskMetaDisplay  *tview.TextView
	editorHint       *tview.TextView
	taskDate         *tview.InputField
	taskStatusToggle *tview.Button
	taskDetailView   *femto.View
	mediaRow         *tview.Flex
	imagePreview     *ImagePreview
	colorScheme      femto.Colorscheme
	taskRepo         repository.TaskRepository
	task             *model.Task
}

// NewTaskDetailPane initializes and configures a TaskDetailPane
func NewTaskDetailPane(taskRepo repository.TaskRepository) *TaskDetailPane {
	pane := TaskDetailPane{
		Flex:             tview.NewFlex().SetDirection(tview.FlexRow),
		header:           NewTaskDetailHeader(taskRepo),
		taskDateDisplay:  tview.NewTextView().SetDynamicColors(true),
		taskMetaDisplay:  tview.NewTextView().SetDynamicColors(true),
		taskStatusToggle: makeButton("Complete", nil).SetLabelColor(tcell.ColorLightGray),
		taskRepo:         taskRepo,
	}

	pane.prepareDetailsEditor()
	pane.imagePreview = NewImagePreview()
	pane.configureMediaRow()

	toggleHint := tview.NewTextView().SetTextColor(tcell.ColorDimGray).SetText("<space> to toggle")
	pane.taskStatusToggle.SetSelectedFunc(pane.toggleTaskStatus)

	pane.editorHint = tview.NewTextView().SetText(" e edit  i paste  o PIXELS  O compatibility  ↓↑ scroll").SetTextColor(tcell.ColorDimGray)

	// Prepare static (no external interaction) elements
	editorLabel := tview.NewFlex().
		AddItem(tview.NewTextView().SetText("Task Not[::u]e[::-]:").SetDynamicColors(true), 0, 1, false).
		AddItem(makeButton("edit", func() { pane.activateEditor() }), 6, 0, false)
	editorHelp := tview.NewFlex().
		AddItem(pane.editorHint, 0, 1, false).
		AddItem(tview.NewTextView().SetTextAlign(tview.AlignRight).
			SetText("syntax:markdown (monakai)").
			SetTextColor(tcell.ColorDimGray), 0, 1, false)

	pane.
		AddItem(pane.header, 4, 1, true).
		AddItem(blankCell, 1, 1, false).
		AddItem(pane.makeDateRow(), 1, 1, true).
		AddItem(pane.taskMetaDisplay, 1, 1, false).
		AddItem(blankCell, 1, 1, false).
		AddItem(editorLabel, 1, 1, false).
		AddItem(pane.mediaRow, 15, 4, false).
		AddItem(editorHelp, 1, 1, false).
		AddItem(blankCell, 0, 1, false).
		AddItem(toggleHint, 1, 1, false).
		AddItem(pane.taskStatusToggle, 3, 1, false)

	pane.SetBorder(true).SetTitle("Task Detail")

	return &pane
}

func (td *TaskDetailPane) configureMediaRow() {
	if td.mediaRow == nil {
		td.mediaRow = tview.NewFlex()
	}
	td.mediaRow.Clear().AddItem(td.taskDetailView, 0, 3, false)
	if td.imagePreview != nil && td.imagePreview.HasImages() {
		td.mediaRow.
			AddItem(nil, 1, 0, false).
			AddItem(td.imagePreview, 0, 2, false)
	}
}

func (td *TaskDetailPane) Export() {
	var content bytes.Buffer

	content.WriteString("# " + td.task.Title + " \n")
	if td.taskDate.GetText() != "" {
		content.WriteString("\n> Due Date: " + td.taskDate.GetText() + " \n")
	}
	content.WriteString("\n" + td.task.Details + " \n")

	clipboard.WriteAll(content.String())
	app.SetFocus(td)
	statusBar.showForSeconds("[lime]Task copied as Markdown", 5)
}

func (td *TaskDetailPane) makeDateRow() *tview.Flex {

	td.taskDate = makeLightTextInput("yyyy-mm-dd").
		SetLabel("Set:").
		SetLabelColor(tcell.ColorWhiteSmoke).
		SetFieldWidth(12).
		SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				value := strings.TrimSpace(td.taskDate.GetText())
				if value == "" {
					td.setTaskDate(0, true)
					break
				}
				date, ok := parseDateInput(value, time.Now())
				if !ok {
					statusBar.showForSeconds("[red]Use yyyy-mm-dd, today, tomorrow, or clear the field", 6)
					return
				}
				td.setTaskDate(date.Unix(), true)
			case tcell.KeyEsc:
				td.setTaskDate(td.task.DueDate, false)
			}
			app.SetFocus(td)
		})

	todaySelector := func() {
		td.setTaskDate(parseDateInputOrCurrent("").Unix(), true)
	}

	nextDaySelector := func() {
		td.setTaskDate(parseDateInputOrCurrent(td.taskDate.GetText()).AddDate(0, 0, 1).Unix(), true)
	}

	prevDaySelector := func() {
		td.setTaskDate(parseDateInputOrCurrent(td.taskDate.GetText()).AddDate(0, 0, -1).Unix(), true)
	}

	return tview.NewFlex().
		AddItem(td.taskDateDisplay, 0, 2, true).
		AddItem(td.taskDate, 14, 0, true).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("today", todaySelector), 8, 1, false).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("+1", nextDaySelector), 4, 1, false).
		AddItem(blankCell, 1, 0, false).
		AddItem(makeButton("-1", prevDaySelector), 4, 1, false)
}

func (td *TaskDetailPane) updateToggleDisplay() {
	if td.task.Completed {
		td.taskStatusToggle.SetLabel("Resume").SetBackgroundColor(tcell.ColorMaroon)
	} else {
		td.taskStatusToggle.SetLabel("Complete").SetBackgroundColor(tcell.ColorDarkGreen)
	}
}

func (td *TaskDetailPane) toggleTaskStatus() {
	message, err := toggleTaskState(td.task, td.taskRepo, time.Now())
	if err != nil {
		statusBar.showForSeconds("[red]Could not update task: "+err.Error(), 5)
		return
	}
	td.setTaskDate(td.task.DueDate, false)
	td.updateToggleDisplay()
	td.updateMetadataDisplay()
	taskPane.RefreshCurrentTaskListing()
	statusBar.showForSeconds("[lime]"+message, 4)
}

func (td *TaskDetailPane) updateMetadataDisplay() {
	if td.task == nil {
		td.taskMetaDisplay.SetText("")
		return
	}
	pin := "no"
	if td.task.Pinned {
		pin = "[deepskyblue]yes[white]"
	}
	recurrence := td.task.Recurrence
	if recurrence == "" {
		recurrence = "none"
	}
	td.taskMetaDisplay.SetText(fmt.Sprintf("[::u]P[::-]riority: %s   [::u]T[::-]ags: [steelblue]%s[white]   Pin: %s   [::u]R[::-]epeat: %s", priorityLabel(td.task.Priority), tview.Escape(taskTagsText(td.task.Tags)), pin, recurrence))
}

func (td *TaskDetailPane) cyclePriority() {
	previous := td.task.Priority
	td.task.Priority = (previous + 1) % 4
	if err := td.taskRepo.Update(td.task); err != nil {
		td.task.Priority = previous
		statusBar.showForSeconds("[red]Could not update priority: "+err.Error(), 5)
		return
	}
	td.updateMetadataDisplay()
	taskPane.RefreshCurrentTaskListing()
	statusBar.showForSeconds("[yellow]Priority: "+priorityLabel(td.task.Priority), 3)
}

func (td *TaskDetailPane) togglePin() {
	previous := td.task.Pinned
	td.task.Pinned = !previous
	if err := td.taskRepo.Update(td.task); err != nil {
		td.task.Pinned = previous
		statusBar.showForSeconds("[red]Could not update pin: "+err.Error(), 5)
		return
	}
	td.updateMetadataDisplay()
	taskPane.RefreshCurrentTaskListing()
}

func (td *TaskDetailPane) editTags() {
	showTextPrompt("Edit tags", "Tags: ", strings.Join(td.task.Tags, " "), func(value string) {
		previous := td.task.Tags
		td.task.Tags = parseTags(value)
		if err := td.taskRepo.Update(td.task); err != nil {
			td.task.Tags = previous
			statusBar.showForSeconds("[red]Could not update tags: "+err.Error(), 5)
			return
		}
		td.updateMetadataDisplay()
		taskPane.RefreshCurrentTaskListing()
		statusBar.showForSeconds("[lime]Tags updated", 3)
	})
}

func (td *TaskDetailPane) cycleRecurrence() {
	values := []string{"", "daily", "weekly", "monthly", "yearly"}
	next := 0
	for i, value := range values {
		if value == td.task.Recurrence {
			next = (i + 1) % len(values)
			break
		}
	}
	previous := td.task.Recurrence
	td.task.Recurrence = values[next]
	if err := td.taskRepo.Update(td.task); err != nil {
		td.task.Recurrence = previous
		statusBar.showForSeconds("[red]Could not update recurrence: "+err.Error(), 5)
		return
	}
	td.updateMetadataDisplay()
	display := td.task.Recurrence
	if display == "" {
		display = "none"
	}
	statusBar.showForSeconds("[yellow]Recurrence: "+display, 3)
}

func (td *TaskDetailPane) openImageViewer() {
	if !td.imagePreview.HasImages() {
		statusBar.showForSeconds("[yellow]This task has no pasted images", 4)
		return
	}
	td.imagePreview.Fit()
	hint := tview.NewTextView().SetText(" n PIXELS  b sextant/color  1-9 zoom  arrows/hjkl pan  f fit  [] images  q close").SetTextColor(tcell.ColorDimGray).SetTextAlign(tview.AlignCenter).SetWrap(false)
	viewer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(td.imagePreview, 0, 1, true).
		AddItem(hint, 1, 0, false)
	td.imagePreview.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			td.imagePreview.Fit()
			closeOverlay()
			return nil
		case tcell.KeyLeft:
			td.imagePreview.Pan(-0.1, 0)
			return nil
		case tcell.KeyRight:
			td.imagePreview.Pan(0.1, 0)
			return nil
		case tcell.KeyDown:
			td.imagePreview.Pan(0, 0.1)
			return nil
		case tcell.KeyUp:
			td.imagePreview.Pan(0, -0.1)
			return nil
		}
		switch event.Rune() {
		case 'q':
			td.imagePreview.Fit()
			closeOverlay()
		case '+', '=':
			td.imagePreview.Zoom(1)
		case '-':
			td.imagePreview.Zoom(-1)
		case 'f', '0':
			td.imagePreview.Fit()
		case 'b':
			td.imagePreview.ToggleDetailMode()
		case 'n':
			td.openNativeImage()
		case 'h':
			td.imagePreview.Pan(-0.1, 0)
		case 'l':
			td.imagePreview.Pan(0.1, 0)
		case 'j':
			td.imagePreview.Pan(0, 0.1)
		case 'k':
			td.imagePreview.Pan(0, -0.1)
		case '[':
			td.imagePreview.Move(-1)
		case ']':
			td.imagePreview.Move(1)
		default:
			if event.Rune() >= '1' && event.Rune() <= '9' {
				td.imagePreview.SetZoom(int(event.Rune() - '0'))
				return nil
			}
			return event
		}
		return nil
	})
	showFullOverlay(viewer, td.imagePreview)
}

func (td *TaskDetailPane) openNativeImage() bool {
	if !td.imagePreview.HasImages() {
		statusBar.showForSeconds("[yellow]This task has no pasted images", 4)
		return false
	}
	if !nativeImageSupported() {
		statusBar.showForSeconds("[yellow]No Sixel, Kitty, or iTerm image protocol detected; opening compatibility mode", 6)
		return false
	}
	var displayErr error
	app.Suspend(func() {
		displayErr = displayNativeImage(td.imagePreview.CurrentPath(), terminalColumns, terminalRows)
	})
	if displayErr != nil {
		statusBar.showForSeconds("[red]Could not display native image: "+displayErr.Error(), 7)
		return false
	}
	return true
}

func (td *TaskDetailPane) openBestImageViewer() {
	if !td.openNativeImage() {
		td.openImageViewer()
	}
}

// Display Task date in detail pane, and update date if asked to
func (td *TaskDetailPane) setTaskDate(unixDate int64, update bool) {
	if update {
		previous := td.task.DueDate
		td.task.DueDate = unixDate
		if err := td.taskRepo.UpdateField(td.task, "DueDate", unixDate); err != nil {
			td.task.DueDate = previous
			statusBar.showForSeconds("Could not update due date: "+err.Error(), 5)
			return
		}
	}

	if unixDate != 0 {
		due := time.Unix(unixDate, 0)
		color := "white"
		humanDate := due.Format(dateLayoutHuman)

		if toDate(due).Before(toDate(time.Now())) {
			color = "red"
		}
		td.taskDateDisplay.SetText(fmt.Sprintf("[::u]D[::-]ue: [%s]%s", color, humanDate))
		td.taskDate.SetText(due.Format(dateLayoutISO))
	} else {
		td.taskDate.SetText("")
		td.taskDateDisplay.SetText("[::u]D[::-]ue: [::d]Not Set")
	}
}

func (td *TaskDetailPane) prepareDetailsEditor() {

	td.taskDetailView = femto.NewView(makeBufferFromString(""))
	td.taskDetailView.SetRuntimeFiles(runtime.Files)

	// var colorScheme femto.Colorscheme
	if monokai := runtime.Files.FindFile(femto.RTColorscheme, "monokai"); monokai != nil {
		if data, err := monokai.Data(); err == nil {
			td.colorScheme = femto.ParseColorscheme(string(data))
		}
	}

	td.taskDetailView.SetColorscheme(td.colorScheme)
	td.taskDetailView.SetBorder(true)
	td.taskDetailView.SetBorderColor(tcell.ColorLightSlateGray)

	td.taskDetailView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			td.updateTaskNote(td.taskDetailView.Buf.String())
			td.deactivateEditor()
			return nil
		case tcell.KeyCtrlV:
			if td.pasteClipboardImage(false) {
				return nil
			}
		}

		return event
	})
}

// pasteClipboardImage stores an image from the system clipboard, inserts a
// Markdown reference at the note cursor, and refreshes the preview. It returns
// false when Ctrl+V should continue through femto's normal text-paste path.
func (td *TaskDetailPane) pasteClipboardImage(showEmptyClipboardMessage bool) bool {
	if td.task == nil {
		return false
	}

	clipboardImage, err := readClipboardImage()
	if err != nil {
		if errors.Is(err, errNoClipboardImage) || errors.Is(err, errClipboardImageUnsupported) {
			if showEmptyClipboardMessage {
				statusBar.showForSeconds("[yellow]"+err.Error(), 5)
			}
			return false
		}
		statusBar.showForSeconds("[red]Could not paste image: "+err.Error(), 7)
		return true
	}

	path, err := saveTaskImage(td.task, clipboardImage)
	if err != nil {
		statusBar.showForSeconds("[red]Could not save pasted image: "+err.Error(), 7)
		return true
	}

	if td.taskDetailView.Readonly {
		td.taskDetailView.Cursor.GotoLoc(td.taskDetailView.Buf.End())
	}
	markdown := imageMarkdown(path)
	if td.taskDetailView.Buf.String() != "" {
		markdown = "\n\n" + markdown + "\n\n"
	}
	td.taskDetailView.Buf.Insert(td.taskDetailView.Cursor.Loc, markdown)
	td.task.Details = td.taskDetailView.Buf.String()
	td.task.Images = append(td.task.Images, path)
	if err := td.taskRepo.Update(td.task); err != nil {
		statusBar.showForSeconds("[red]Image was pasted but the task could not be saved: "+err.Error(), 7)
		return true
	}

	td.imagePreview.SetImages(td.task.Images)
	td.configureMediaRow()
	statusBar.showForSeconds("[lime]Image pasted — press lowercase o for real pixels (uppercase O is compatibility mode)", 7)
	return true
}

func (td *TaskDetailPane) updateTaskNote(note string) {
	td.task.Details = note
	err := td.taskRepo.Update(td.task)
	if err == nil {
		statusBar.showForSeconds("[lime]Saved task detail", 5)
	} else {
		statusBar.showForSeconds("[red]Could not save: "+err.Error(), 5)
	}
}

func makeBufferFromString(content string) *femto.Buffer {
	buff := femto.NewBufferFromString(content, "")
	// taskDetail.Settings["ruler"] = false
	buff.Settings["filetype"] = "markdown"
	buff.Settings["keepautoindent"] = true
	buff.Settings["statusline"] = false
	buff.Settings["softwrap"] = true
	buff.Settings["scrollbar"] = true

	return buff
}

func (td *TaskDetailPane) activateEditor() {
	td.taskDetailView.Readonly = false
	td.taskDetailView.SetBorderColor(tcell.ColorDarkOrange)
	td.editorHint.SetText(" Esc to save changes")
	app.SetFocus(td.taskDetailView)
}

func (td *TaskDetailPane) deactivateEditor() {
	td.taskDetailView.Readonly = true
	td.taskDetailView.SetBorderColor(tcell.ColorLightSlateGray)
	td.editorHint.SetText(" e edit  i paste  o PIXELS  O compatibility  ↓↑ scroll")
	app.SetFocus(td)
}

func (td *TaskDetailPane) editInExternalEditor() {

	tmpFileName, err := writeToTmpFile(td.task.Details)
	if err != nil {
		statusBar.showForSeconds("[red::]Failed to create tmp file. Try in-app editing by pressing e", 5)
		return
	}

	var messageToShow, updatedContent string
	contentLoaded := false
	app.Suspend(func() {
		editor := util.GetEnvStr("EDITOR", "")
		if editor == "" {
			if goruntime.GOOS == "windows" {
				editor = "notepad"
			} else {
				editor = "vim"
			}
		}
		cmd := exec.Command(editor, tmpFileName)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			messageToShow = "[red::]Failed to save content. Try in-app editing by pressing e"
			return
		}

		if content, readErr := ioutil.ReadFile(tmpFileName); readErr == nil {
			updatedContent = string(content)
			contentLoaded = true
		} else {
			messageToShow = "[red::]Failed to load external editing. Try in-app editing by pressing e"
		}
	})

	if messageToShow != "" {
		statusBar.showForSeconds(messageToShow, 10)
	}

	if contentLoaded {
		td.updateTaskNote(updatedContent)
		td.SetTask(td.task)
	}

	app.EnableMouse(true)

	_ = os.Remove(tmpFileName)

	// app.SetFocus(td)
}

// writeToTmpFile writes content to a new temp file, using a single handle to avoid Windows file-locking issues.
func writeToTmpFile(content string) (string, error) {
	tmpFile, err := ioutil.TempFile("", "geek_life_task_note_*.md")
	if err != nil {
		return "", err
	}
	fileName := tmpFile.Name()
	_, err = tmpFile.WriteString(content)
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return fileName, err
}

func (td *TaskDetailPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlV:
		td.pasteClipboardImage(true)
		return nil
	case tcell.KeyEsc:
		app.SetFocus(taskPane)
		return nil
	case tcell.KeyDown:
		td.taskDetailView.ScrollDown(1)
		return nil
	case tcell.KeyUp:
		td.taskDetailView.ScrollUp(1)
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'e':
			td.activateEditor()
			return nil
		case 'i':
			td.pasteClipboardImage(true)
			return nil
		case 'o':
			td.openBestImageViewer()
			return nil
		case 'O':
			td.openImageViewer()
			return nil
		case '!':
			td.cyclePriority()
			return nil
		case 's':
			td.togglePin()
			return nil
		case 'g':
			td.editTags()
			return nil
		case 'R':
			td.cycleRecurrence()
			return nil
		case 'v':
			td.editInExternalEditor()
			return nil
		case 'd':
			app.SetFocus(td.taskDate)
			return nil
		case 'r':
			td.header.ShowRename()
			return nil
		case ' ':
			td.toggleTaskStatus()
			return nil
		case 'D':
			taskPane.DeleteCurrentTask()
			return nil
		case 'x':
			td.Export()
			return nil
		case '[':
			td.imagePreview.Move(-1)
			return nil
		case ']':
			td.imagePreview.Move(1)
			return nil
		}
	}

	return event
}

// SetTask sets a Task to be displayed
func (td *TaskDetailPane) SetTask(task *model.Task) {
	// Auto-save if the in-app editor is active so navigating away doesn't discard changes
	if td.task != nil && !td.taskDetailView.Readonly {
		td.updateTaskNote(td.taskDetailView.Buf.String())
	}
	td.task = task

	td.header.SetTask(task)
	td.imagePreview.SetImages(task.Images)
	td.configureMediaRow()
	td.taskDetailView.Buf = makeBufferFromString(td.task.Details)
	td.taskDetailView.SetColorscheme(td.colorScheme)
	td.taskDetailView.Start()
	td.setTaskDate(td.task.DueDate, false)
	td.updateToggleDisplay()
	td.updateMetadataDisplay()
	td.deactivateEditor()
}

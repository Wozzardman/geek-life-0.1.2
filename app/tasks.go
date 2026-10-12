package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

const (
	taskBodyList    = "list"
	taskBodyHint    = "hint"
	taskInputNone   = "none"
	taskInputNew    = "new"
	taskInputSearch = "search"
)

// TaskPane displays tasks, smart lists, quick capture, and global search.
type TaskPane struct {
	*tview.Flex
	list       *tview.List
	body       *tview.Pages
	footer     *tview.Pages
	tasks      []model.Task
	activeTask *model.Task

	newTask      *tview.InputField
	searchInput  *tview.InputField
	searchPool   []model.Task
	searchTitles map[int64]string
	searchActive bool
	projectRepo  repository.ProjectRepository
	taskRepo     repository.TaskRepository
	hint         *tview.TextView
	viewMode     string
	viewProject  int64
	isTrash      bool
	lastDeleted  *model.Task
}

// NewTaskPane initializes and configures a TaskPane.
func NewTaskPane(projectRepo repository.ProjectRepository, taskRepo repository.TaskRepository) *TaskPane {
	pane := TaskPane{
		Flex:         tview.NewFlex().SetDirection(tview.FlexRow),
		list:         tview.NewList().ShowSecondaryText(false),
		body:         tview.NewPages(),
		footer:       tview.NewPages(),
		newTask:      makeLightTextInput("Task title  #tag  !1-3  @today  ~weekly  +pin"),
		searchInput:  makeLightTextInput("/ search; filters: #tag @project !2 is:done due:overdue"),
		projectRepo:  projectRepo,
		taskRepo:     taskRepo,
		hint:         tview.NewTextView().SetTextColor(tcell.ColorYellow).SetTextAlign(tview.AlignCenter),
		searchTitles: make(map[int64]string),
	}

	pane.list.SetSelectedBackgroundColor(tcell.ColorDarkBlue)
	pane.list.SetDoneFunc(func() {
		if pane.searchActive {
			pane.CloseSearch()
			return
		}
		app.SetFocus(projectPane)
	})
	pane.bindNewTaskInput()
	pane.bindSearchInput()

	pane.body.
		AddPage(taskBodyList, pane.list, true, false).
		AddPage(taskBodyHint, pane.hint, true, true)
	pane.footer.
		AddPage(taskInputNone, tview.NewBox(), true, true).
		AddPage(taskInputNew, pane.newTask, true, false).
		AddPage(taskInputSearch, pane.searchInput, true, false)
	pane.
		AddItem(pane.body, 0, 1, true).
		AddItem(pane.footer, 1, 0, false)

	pane.SetBorder(true).SetTitle("[::u]T[::-]asks")
	pane.setHintMessage()
	return &pane
}

func (pane *TaskPane) bindNewTaskInput() {
	pane.newTask.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			project := projectPane.GetActiveProject()
			if project == nil || project.Archived {
				statusBar.showForSeconds("[yellow]Select an active project before creating a task", 5)
				return
			}
			spec, err := parseQuickTaskInput(pane.newTask.GetText(), time.Now())
			if err != nil {
				statusBar.showForSeconds("[red]"+err.Error(), 5)
				return
			}
			task, err := pane.taskRepo.Create(*project, spec.Title, "", "", spec.DueDate)
			if err != nil {
				statusBar.showForSeconds("[red]Could not create task: "+err.Error(), 5)
				return
			}
			createdTask := task
			task.Tags, task.Priority, task.Recurrence, task.Pinned = spec.Tags, spec.Priority, spec.Recurrence, spec.Pinned
			metadataErr := pane.taskRepo.Update(&task)
			if metadataErr != nil {
				task = createdTask
			}
			pane.tasks = append(pane.tasks, task)
			pane.addTaskToList(len(pane.tasks) - 1)
			pane.list.SetCurrentItem(len(pane.tasks) - 1)
			pane.newTask.SetText("")
			if metadataErr != nil {
				statusBar.showForSeconds("[yellow]Task created, but its metadata could not be saved: "+metadataErr.Error(), 7)
			} else {
				statusBar.showForSeconds("[lime]Task created — keep typing or press Esc", 4)
			}
		case tcell.KeyEsc:
			app.SetFocus(pane.list)
		}
	})
}

func (pane *TaskPane) bindSearchInput() {
	pane.searchInput.SetChangedFunc(func(query string) { pane.applySearch(query) })
	pane.searchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if len(pane.tasks) == 0 {
				statusBar.showForSeconds("[yellow]No matching tasks", 3)
				return
			}
			app.SetFocus(pane.list)
		case tcell.KeyEsc:
			pane.CloseSearch()
		}
	})
	pane.searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyCtrlN:
			pane.list.SetCurrentItem(pane.list.GetCurrentItem() + 1)
			return nil
		case tcell.KeyUp, tcell.KeyCtrlP:
			pane.list.SetCurrentItem(pane.list.GetCurrentItem() - 1)
			return nil
		}
		return event
	})
}

// ClearList removes all visible task state.
func (pane *TaskPane) ClearList() {
	pane.list.Clear()
	pane.tasks = nil
	pane.activeTask = nil
}

// SetList displays tasks and switches away from the welcome panel.
func (pane *TaskPane) SetList(tasks []model.Task) {
	pane.ClearList()
	pane.tasks = append(pane.tasks, tasks...)
	for i := range pane.tasks {
		pane.addTaskToList(i)
	}
	pane.body.SwitchToPage(taskBodyList)
}

func (pane *TaskPane) addTaskToList(i int) *tview.List {
	return pane.list.AddItem(makeTaskListingTitle(pane.tasks[i]), "", 0, func(taskIndex int) func() {
		return func() { pane.ActivateTask(taskIndex) }
	}(i))
}

func (pane *TaskPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'j':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() + 1)
		return nil
	case 'k':
		pane.list.SetCurrentItem(pane.list.GetCurrentItem() - 1)
		return nil
	case 'h':
		app.SetFocus(projectPane)
		return nil
	case 'n':
		pane.StartNewTask()
		return nil
	case 'D':
		pane.DeleteCurrentTask()
		return nil
	case 'u':
		if pane.isTrash {
			pane.RestoreCurrentTask()
		} else {
			pane.UndoLastDelete()
		}
		return nil
	case '[':
		pane.MoveCurrentTaskUp()
		return nil
	case ']':
		pane.MoveCurrentTaskDown()
		return nil
	case ' ':
		pane.ToggleCurrentTask()
		return nil
	case '!':
		pane.CycleCurrentPriority()
		return nil
	case 's':
		pane.ToggleCurrentPin()
		return nil
	}
	return event
}

func (pane *TaskPane) StartNewTask() {
	project := projectPane.GetActiveProject()
	if project == nil || project.Archived {
		statusBar.showForSeconds("[yellow]Select an active project before creating a task", 5)
		return
	}
	pane.footer.SwitchToPage(taskInputNew)
	app.SetFocus(pane.newTask)
}

// LoadProjectTasks loads tasks belonging to a project.
func (pane *TaskPane) LoadProjectTasks(project model.Project) {
	tasks, err := pane.taskRepo.GetAllByProject(project)
	if err != nil {
		statusBar.showForSeconds("[red]Could not load tasks: "+err.Error(), 5)
		return
	}
	pane.viewMode, pane.viewProject, pane.isTrash, pane.searchActive = "project", project.ID, false, false
	pane.SetList(tasks)
	pane.SetTitle("[::u]T[::-]asks — " + tview.Escape(project.Title))
	if project.Archived {
		pane.footer.SwitchToPage(taskInputNone)
	} else {
		pane.footer.SwitchToPage(taskInputNew)
	}
}

// LoadDynamicList loads one of the built-in smart views.
func (pane *TaskPane) LoadDynamicList(logic string) {
	var tasks []model.Task
	var err error
	if logic == "trash" {
		tasks, err = pane.taskRepo.GetDeleted()
	} else {
		tasks, err = pane.taskRepo.GetAll()
	}
	if err != nil {
		statusBar.showForSeconds("[red]Could not load smart list: "+err.Error(), 5)
		return
	}

	today := toDate(time.Now()).Unix()
	tomorrow := toDate(time.Now()).AddDate(0, 0, 1).Unix()
	week := toDate(time.Now()).AddDate(0, 0, 7).Unix()
	label := "All Open"
	filtered := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		include := false
		switch logic {
		case "today":
			label, include = "Today", !task.Completed && task.DueDate == today
		case "overdue":
			label, include = "Overdue", !task.Completed && task.DueDate != 0 && task.DueDate < today
		case "tomorrow":
			label, include = "Tomorrow", !task.Completed && task.DueDate == tomorrow
		case "upcoming":
			label, include = "Upcoming 7 Days", !task.Completed && task.DueDate > today && task.DueDate <= week
		case "unscheduled":
			label, include = "Unscheduled", !task.Completed && task.DueDate == 0
		case "completed":
			label, include = "Completed", task.Completed
		case "pinned":
			label, include = "Pinned", !task.Completed && task.Pinned
		case "high":
			label, include = "High Priority", !task.Completed && task.Priority == 3
		case "trash":
			label, include = "Trash", true
		default:
			logic, label, include = "all", "All Open", !task.Completed
		}
		if include {
			filtered = append(filtered, task)
		}
	}
	sortTasksForDisplay(filtered)
	projectPane.activeProject = nil
	pane.viewMode, pane.viewProject, pane.isTrash, pane.searchActive = logic, 0, logic == "trash", false
	pane.SetList(filtered)
	pane.footer.SwitchToPage(taskInputNone)
	pane.SetTitle(fmt.Sprintf("[::u]T[::-]asks — %s (%d)", label, len(filtered)))
	removeThirdCol()
	app.SetFocus(pane.list)
	statusBar.showForSeconds(fmt.Sprintf("[yellow]%s: %d task(s)", label, len(filtered)), 3)
}

func sortTasksForDisplay(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Pinned != tasks[j].Pinned {
			return tasks[i].Pinned
		}
		if tasks[i].Completed != tasks[j].Completed {
			return !tasks[i].Completed
		}
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		if tasks[i].DueDate == 0 {
			return false
		}
		if tasks[j].DueDate == 0 {
			return true
		}
		return tasks[i].DueDate < tasks[j].DueDate
	})
}

// ShowSearch opens cross-project structured search. Results update as you type.
func (pane *TaskPane) ShowSearch() {
	tasks, err := pane.taskRepo.GetAll()
	if err != nil {
		statusBar.showForSeconds("[red]Could not start search: "+err.Error(), 5)
		return
	}
	pane.searchPool = tasks
	pane.searchTitles = make(map[int64]string)
	projects, _ := pane.projectRepo.GetAll()
	archived, _ := pane.projectRepo.GetAllArchived()
	for _, project := range append(projects, archived...) {
		pane.searchTitles[project.ID] = project.Title
	}
	pane.searchActive, pane.viewMode, pane.viewProject, pane.isTrash = true, "search", 0, false
	projectPane.activeProject = nil
	pane.footer.SwitchToPage(taskInputSearch)
	pane.searchInput.SetText("")
	pane.applySearch("")
	app.SetFocus(pane.searchInput)
}

func (pane *TaskPane) applySearch(query string) {
	if !pane.searchActive {
		return
	}
	results := make([]model.Task, 0)
	for _, task := range pane.searchPool {
		if taskMatchesQuery(task, pane.searchTitles[task.ProjectID], query, time.Now()) {
			results = append(results, task)
		}
	}
	sortTasksForDisplay(results)
	pane.SetList(results)
	pane.SetTitle(fmt.Sprintf("[::u]T[::-]asks — Search (%d)", len(results)))
}

// CloseSearch returns to the all-open smart list.
func (pane *TaskPane) CloseSearch() {
	pane.searchActive = false
	pane.searchInput.SetText("")
	pane.LoadDynamicList("all")
}

// ActivateTask loads task details.
func (pane *TaskPane) ActivateTask(idx int) {
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	removeThirdCol()
	pane.activeTask = &pane.tasks[idx]
	taskDetailPane.SetTask(pane.activeTask)
	contents.AddItem(taskDetailPane, 0, 3, false)
	app.SetFocus(taskDetailPane)
}

// ClearCompletedTasks moves completed tasks in the current view to trash.
func (pane *TaskPane) ClearCompletedTasks() {
	survivors := make([]model.Task, 0, len(pane.tasks))
	count := 0
	for i := range pane.tasks {
		if !pane.tasks[i].Completed {
			survivors = append(survivors, pane.tasks[i])
			continue
		}
		pane.tasks[i].DeletedAt = time.Now().Unix()
		if err := pane.taskRepo.Update(&pane.tasks[i]); err != nil {
			survivors = append(survivors, pane.tasks[i])
			continue
		}
		copyOfTask := pane.tasks[i]
		pane.lastDeleted = &copyOfTask
		count++
	}
	pane.SetList(survivors)
	statusBar.showForSeconds(fmt.Sprintf("[yellow]%d completed task(s) moved to Trash; u = undo last", count), 5)
}

// ReloadCurrentTask refreshes the current listing and detail display.
func (pane *TaskPane) ReloadCurrentTask() {
	if pane.activeTask == nil {
		return
	}
	pane.RefreshCurrentTaskListing()
	taskDetailPane.SetTask(pane.activeTask)
}

func (pane *TaskPane) RefreshCurrentTaskListing() {
	idx := pane.list.GetCurrentItem()
	if pane.activeTask == nil || idx < 0 || idx >= len(pane.tasks) {
		return
	}
	pane.list.SetItemText(idx, makeTaskListingTitle(*pane.activeTask), "")
}

// DeleteCurrentTask moves an active task to Trash, or permanently deletes a
// task that is already in Trash after confirmation.
func (pane *TaskPane) DeleteCurrentTask() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	task := &pane.tasks[idx]
	if pane.isTrash || task.DeletedAt != 0 {
		title := task.Title
		showConfirm("Permanently delete task?", "This cannot be undone and also removes its stored images:\n\n"+tview.Escape(title), func() {
			if err := pane.taskRepo.Delete(task); err != nil {
				statusBar.showForSeconds("[red]Could not permanently delete task: "+err.Error(), 5)
				return
			}
			attachmentErr := removeTaskAttachments(task.ID)
			pane.removeTaskAt(idx)
			if attachmentErr != nil {
				statusBar.showForSeconds("[yellow]Task deleted, but its attachment directory could not be removed: "+attachmentErr.Error(), 7)
			} else {
				statusBar.showForSeconds("[lime]Task and stored images permanently deleted", 4)
			}
		})
		return
	}

	task.DeletedAt = time.Now().Unix()
	if err := pane.taskRepo.Update(task); err != nil {
		task.DeletedAt = 0
		statusBar.showForSeconds("[red]Could not move task to Trash: "+err.Error(), 5)
		return
	}
	copyOfTask := *task
	pane.lastDeleted = &copyOfTask
	pane.removeTaskAt(idx)
	statusBar.showForSeconds("[yellow]Task moved to Trash — press u to undo", 5)
}

func (pane *TaskPane) removeTaskAt(idx int) {
	pane.tasks = append(pane.tasks[:idx], pane.tasks[idx+1:]...)
	pane.rebuildList()
	removeThirdCol()
	if idx >= len(pane.tasks) {
		idx = len(pane.tasks) - 1
	}
	if idx >= 0 {
		pane.list.SetCurrentItem(idx)
	}
	app.SetFocus(pane.list)
}

func (pane *TaskPane) UndoLastDelete() {
	if pane.lastDeleted == nil {
		statusBar.showForSeconds("[yellow]Nothing to undo", 3)
		return
	}
	pane.lastDeleted.DeletedAt = 0
	if err := pane.taskRepo.Update(pane.lastDeleted); err != nil {
		statusBar.showForSeconds("[red]Could not restore task: "+err.Error(), 5)
		return
	}
	pane.lastDeleted = nil
	pane.ReloadCurrentView()
	statusBar.showForSeconds("[lime]Task restored", 4)
}

func (pane *TaskPane) RestoreCurrentTask() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	previousDeletedAt := pane.tasks[idx].DeletedAt
	pane.tasks[idx].DeletedAt = 0
	if err := pane.taskRepo.Update(&pane.tasks[idx]); err != nil {
		pane.tasks[idx].DeletedAt = previousDeletedAt
		statusBar.showForSeconds("[red]Could not restore task: "+err.Error(), 5)
		return
	}
	pane.removeTaskAt(idx)
	statusBar.showForSeconds("[lime]Task restored from Trash", 4)
}

func (pane *TaskPane) ReloadCurrentView() {
	if pane.searchActive {
		pane.ShowSearch()
		return
	}
	if pane.viewMode == "project" && pane.viewProject != 0 {
		project, err := pane.projectRepo.GetByID(pane.viewProject)
		if err == nil {
			projectPane.activeProject = &project
			pane.LoadProjectTasks(project)
			app.SetFocus(pane.list)
			return
		}
	}
	pane.LoadDynamicList(pane.viewMode)
}

func (pane *TaskPane) ToggleCurrentTask() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	message, err := toggleTaskState(&pane.tasks[idx], pane.taskRepo, time.Now())
	if err != nil {
		statusBar.showForSeconds("[red]Could not update task: "+err.Error(), 5)
		return
	}
	pane.list.SetItemText(idx, makeTaskListingTitle(pane.tasks[idx]), "")
	statusBar.showForSeconds("[lime]"+message, 4)
}

func (pane *TaskPane) CycleCurrentPriority() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	previous := pane.tasks[idx].Priority
	pane.tasks[idx].Priority = (previous + 1) % 4
	if err := pane.taskRepo.Update(&pane.tasks[idx]); err != nil {
		pane.tasks[idx].Priority = previous
		statusBar.showForSeconds("[red]Could not update priority: "+err.Error(), 5)
		return
	}
	pane.list.SetItemText(idx, makeTaskListingTitle(pane.tasks[idx]), "")
	statusBar.showForSeconds("[yellow]Priority: "+priorityLabel(pane.tasks[idx].Priority), 3)
}

func (pane *TaskPane) ToggleCurrentPin() {
	idx := pane.list.GetCurrentItem()
	if idx < 0 || idx >= len(pane.tasks) {
		return
	}
	previous := pane.tasks[idx].Pinned
	pane.tasks[idx].Pinned = !previous
	if err := pane.taskRepo.Update(&pane.tasks[idx]); err != nil {
		pane.tasks[idx].Pinned = previous
		statusBar.showForSeconds("[red]Could not update pin: "+err.Error(), 5)
		return
	}
	pane.list.SetItemText(idx, makeTaskListingTitle(pane.tasks[idx]), "")
	statusBar.showForSeconds("[yellow]Pin updated", 3)
}

// MoveCurrentTaskUp moves a project task one position up.
func (pane *TaskPane) MoveCurrentTaskUp() {
	idx := pane.list.GetCurrentItem()
	if pane.viewMode != "project" || idx <= 0 || idx >= len(pane.tasks) {
		return
	}
	pane.tasks[idx], pane.tasks[idx-1] = pane.tasks[idx-1], pane.tasks[idx]
	pane.persistTaskOrder(idx, idx-1)
	pane.rebuildList()
	pane.list.SetCurrentItem(idx - 1)
}

// MoveCurrentTaskDown moves a project task one position down.
func (pane *TaskPane) MoveCurrentTaskDown() {
	idx := pane.list.GetCurrentItem()
	if pane.viewMode != "project" || idx < 0 || idx >= len(pane.tasks)-1 {
		return
	}
	pane.tasks[idx], pane.tasks[idx+1] = pane.tasks[idx+1], pane.tasks[idx]
	pane.persistTaskOrder(idx, idx+1)
	pane.rebuildList()
	pane.list.SetCurrentItem(idx + 1)
}

func (pane *TaskPane) persistTaskOrder(first, second int) {
	pane.tasks[first].Order, pane.tasks[second].Order = first, second
	if err := pane.taskRepo.Update(&pane.tasks[first]); err != nil {
		statusBar.showForSeconds("[red]Could not save task order: "+err.Error(), 5)
		return
	}
	if err := pane.taskRepo.Update(&pane.tasks[second]); err != nil {
		statusBar.showForSeconds("[red]Could not save task order: "+err.Error(), 5)
		return
	}
	statusBar.showForSeconds("[yellow]Task order updated", 2)
}

func (pane *TaskPane) rebuildList() {
	pane.list.Clear()
	for i := range pane.tasks {
		pane.addTaskToList(i)
	}
}

func (pane *TaskPane) setHintMessage() {
	if len(projectPane.projects) == 0 {
		pane.hint.SetText("Welcome to Geek-life\n\nCreate a project with p, n\nOpen commands with : or Ctrl+P\nSearch every task with /\nShow keyboard help with ?")
	} else {
		pane.hint.SetText("Select a project or smart list\n\n: commands   / search   ? help\nEverything is keyboard accessible")
	}
}

func toggleTaskState(task *model.Task, repo repository.TaskRepository, now time.Time) (string, error) {
	if task.DeletedAt != 0 {
		return "", fmt.Errorf("restore the task before changing it")
	}
	original := *task
	if !task.Completed && validRecurrence(task.Recurrence) {
		task.LastCompletedAt = now.Unix()
		task.DueDate = nextRecurringDue(*task, now)
		task.CompletedAt = 0
		if err := repo.Update(task); err != nil {
			*task = original
			return "", err
		}
		return fmt.Sprintf("Recurring task advanced to %s", time.Unix(task.DueDate, 0).Format(dateLayoutISO)), nil
	}

	task.Completed = !task.Completed
	if task.Completed {
		task.CompletedAt, task.LastCompletedAt = now.Unix(), now.Unix()
	} else {
		task.CompletedAt = 0
	}
	if err := repo.Update(task); err != nil {
		*task = original
		return "", err
	}
	if task.Completed {
		return "Task completed", nil
	}
	return "Task resumed", nil
}

func taskTagsText(tags []string) string {
	if len(tags) == 0 {
		return "none"
	}
	values := make([]string, len(tags))
	for i, tag := range tags {
		values[i] = "#" + tag
	}
	return strings.Join(values, " ")
}

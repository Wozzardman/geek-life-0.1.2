package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

// ProjectPane Displays projects and dynamic lists
type ProjectPane struct {
	*tview.Flex
	projects            []model.Project
	archivedProjects    []model.Project
	list                *tview.List
	newProject          *tview.InputField
	repo                repository.ProjectRepository
	activeProject       *model.Project
	projectListStarting int // The index in list where project names starts
	selectable          map[int]bool
	defaultSelection    int
}

// NewProjectPane initializes
func NewProjectPane(repo repository.ProjectRepository) *ProjectPane {
	pane := ProjectPane{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		list:       tview.NewList().ShowSecondaryText(false),
		newProject: makeLightTextInput("+[New Project]"),
		repo:       repo,
		selectable: make(map[int]bool),
	}

	pane.newProject.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			pane.addNewProject()
		case tcell.KeyEsc:
			app.SetFocus(projectPane)
		}
	})

	pane.AddItem(pane.list, 0, 1, true).
		AddItem(pane.newProject, 1, 0, false)

	pane.SetBorder(true).SetTitle("[::u]P[::-]rojects")
	pane.loadListItems(false)

	return &pane
}

func (pane *ProjectPane) addNewProject() {
	name := strings.TrimSpace(pane.newProject.GetText())
	if len(name) < 3 {
		statusBar.showForSeconds("[red::]Project name should be at least 3 character", 5)
		return
	}

	project, err := pane.repo.Create(name, "")
	if err != nil {
		statusBar.showForSeconds("[red::]Failed to create Project:"+err.Error(), 5)
	} else {
		statusBar.showForSeconds(fmt.Sprintf("[yellow::]Project %s created. Press n to start adding new tasks.", tview.Escape(name)), 10)
		pane.projects = append(pane.projects, project)
		pane.addProjectToList(len(pane.projects)-1, true)
		pane.newProject.SetText("")
	}
}

func (pane *ProjectPane) addDynamicLists() {
	pane.addSection("Smart Lists")
	pane.addSelectableItem("≡ All Open", func() { taskPane.LoadDynamicList("all") })
	pane.defaultSelection = pane.list.GetItemCount()
	pane.addSelectableItem("◉ Today", func() { taskPane.LoadDynamicList("today") })
	pane.addSelectableItem("⚠ Overdue", func() { taskPane.LoadDynamicList("overdue") })
	pane.addSelectableItem("▷ Tomorrow", func() { taskPane.LoadDynamicList("tomorrow") })
	pane.addSelectableItem("→ Upcoming", func() { taskPane.LoadDynamicList("upcoming") })
	pane.addSelectableItem("★ Pinned", func() { taskPane.LoadDynamicList("pinned") })
	pane.addSelectableItem("!! High Priority", func() { taskPane.LoadDynamicList("high") })
	pane.addSelectableItem("✓ Completed", func() { taskPane.LoadDynamicList("completed") })
	pane.addSelectableItem("− Unscheduled", func() { taskPane.LoadDynamicList("unscheduled") })
	pane.addSelectableItem("⌫ Trash", func() { taskPane.LoadDynamicList("trash") })
}

func (pane *ProjectPane) addSelectableItem(title string, selected func()) {
	idx := pane.list.GetItemCount()
	pane.selectable[idx] = true
	pane.list.AddItem(title, "", 0, selected)
}

func (pane *ProjectPane) addProjectList() {
	pane.addSection("Projects")
	pane.projectListStarting = pane.list.GetItemCount()

	var err error
	pane.projects, err = pane.repo.GetAll()
	if err != nil {
		statusBar.showForSeconds("Could not load Projects: "+err.Error(), 5)
		return
	}

	for i := range pane.projects {
		pane.addProjectToList(i, false)
	}

	pane.list.SetCurrentItem(pane.defaultSelection)
}

func (pane *ProjectPane) addProjectToList(i int, selectItem bool) {
	// To avoid overriding of loop variables - https://www.calhoun.io/gotchas-and-common-mistakes-with-closures-in-go/
	idx := pane.list.GetItemCount()
	pane.selectable[idx] = true
	pane.list.AddItem("- "+tview.Escape(pane.projects[i].Title), "", 0, func(idx int) func() {
		return func() { pane.activateProject(idx) }
	}(i))

	if selectItem {
		pane.list.SetCurrentItem(-1)
		pane.activateProject(i)
	}
}

func (pane *ProjectPane) addSection(name string) {
	pane.list.AddItem("[::d]"+name, "", 0, nil)
	pane.list.AddItem("[::d]"+strings.Repeat(string(tcell.RuneS3), 25), "", 0, nil)
}

func (pane *ProjectPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyDown, tcell.KeyTab:
		pane.moveSelection(1)
		return nil
	case tcell.KeyUp, tcell.KeyBacktab:
		pane.moveSelection(-1)
		return nil
	}
	switch event.Rune() {
	case 'j':
		pane.moveSelection(1)
		return nil
	case 'k':
		pane.moveSelection(-1)
		return nil
	case 'n':
		app.SetFocus(pane.newProject)
		return nil
	}

	return event
}

func (pane *ProjectPane) moveSelection(delta int) {
	count := pane.list.GetItemCount()
	if count == 0 {
		return
	}
	idx := pane.list.GetCurrentItem()
	for checked := 0; checked < count; checked++ {
		idx = (idx + delta + count) % count
		if pane.selectable[idx] {
			pane.list.SetCurrentItem(idx)
			return
		}
	}
}

func (pane *ProjectPane) activateProject(idx int) {
	pane.activeProject = &pane.projects[idx]
	taskPane.LoadProjectTasks(*pane.activeProject)

	removeThirdCol()
	projectDetailPane.SetProject(pane.activeProject)
	contents.AddItem(projectDetailPane, 25, 0, false)
	app.SetFocus(taskPane)
}

// RemoveActivateProject deletes the currently active project
func (pane *ProjectPane) RemoveActivateProject() {
	if pane.activeProject == nil {
		return
	}
	project := pane.activeProject
	showConfirm("Permanently delete project?", "This permanently deletes the project, all of its tasks, and their stored images:\n\n"+tview.Escape(project.Title)+"\n\nArchiving is the safer option.", func() {
		projectTasks, _ := taskRepo.GetAllByProject(*project)
		deletedTasks, _ := taskRepo.GetDeleted()
		for _, task := range deletedTasks {
			if task.ProjectID == project.ID {
				projectTasks = append(projectTasks, task)
			}
		}
		if err := pane.repo.DeleteWithTasks(project); err != nil {
			statusBar.showForSeconds("[red]Could not delete project: "+err.Error(), 6)
			return
		}
		taskPane.ClearList()
		pane.activeProject = nil
		var attachmentErr error
		for _, task := range projectTasks {
			if err := removeTaskAttachments(task.ID); err != nil && attachmentErr == nil {
				attachmentErr = err
			}
		}
		if attachmentErr != nil {
			statusBar.showForSeconds("[yellow]Project deleted, but some attachments could not be removed: "+attachmentErr.Error(), 7)
		} else {
			statusBar.showForSeconds("[lime]Project, tasks, and stored images permanently deleted", 5)
		}
		removeThirdCol()
		pane.loadListItems(true)
	})
}

func (pane *ProjectPane) loadListItems(focus bool) {
	pane.list.Clear()
	pane.selectable = make(map[int]bool)
	pane.addDynamicLists()
	pane.list.AddItem("", "", 0, nil)
	pane.addProjectList()
	pane.addArchivedSection()

	if focus {
		app.SetFocus(pane)
	}
}

// GetActiveProject provides pointer to currently active project
func (pane *ProjectPane) GetActiveProject() *model.Project {
	return pane.activeProject
}

// RenameActiveProject edits the current project without leaving the task list.
func (pane *ProjectPane) RenameActiveProject() {
	if pane.activeProject == nil {
		statusBar.showForSeconds("[yellow]Select a project before renaming it", 4)
		return
	}
	showTextPrompt("Rename project", "Name: ", pane.activeProject.Title, func(value string) {
		name := strings.TrimSpace(value)
		if len(name) < 3 {
			statusBar.showForSeconds("[red]Project name should be at least 3 characters", 5)
			return
		}
		previous := pane.activeProject.Title
		pane.activeProject.Title = name
		if err := pane.repo.Update(pane.activeProject); err != nil {
			pane.activeProject.Title = previous
			statusBar.showForSeconds("[red]Could not rename project: "+err.Error(), 5)
			return
		}
		idx := pane.list.GetCurrentItem()
		prefix := "- "
		if pane.activeProject.Archived {
			prefix = "[::d]~ "
		}
		pane.list.SetItemText(idx, prefix+tview.Escape(name), "")
		projectDetailPane.SetProject(pane.activeProject)
		if taskPane.viewMode == "project" {
			taskPane.SetTitle("[::u]T[::-]asks — " + tview.Escape(name))
		}
		statusBar.showForSeconds("[lime]Project renamed", 4)
	})
}

// ArchiveActiveProject archives the currently active project (hides it from the main list)
func (pane *ProjectPane) ArchiveActiveProject() {
	if pane.activeProject == nil {
		return
	}
	if err := pane.repo.UpdateField(pane.activeProject, "Archived", true); err != nil {
		statusBar.showForSeconds("[red]Could not archive project: "+err.Error(), 5)
		return
	}
	pane.activeProject.Archived = true
	pane.activeProject = nil
	statusBar.showForSeconds("[yellow]Project archived.", 5)
	taskPane.ClearList()
	removeThirdCol()
	pane.loadListItems(true)
}

// UnarchiveActiveProject restores an archived project back to the active project list
func (pane *ProjectPane) UnarchiveActiveProject() {
	if pane.activeProject == nil {
		return
	}
	if err := pane.repo.UpdateField(pane.activeProject, "Archived", false); err != nil {
		statusBar.showForSeconds("[red]Could not restore project: "+err.Error(), 5)
		return
	}
	pane.activeProject.Archived = false
	pane.activeProject = nil
	statusBar.showForSeconds("[lime]Project restored to active.", 5)
	taskPane.ClearList()
	removeThirdCol()
	pane.loadListItems(true)
}

func (pane *ProjectPane) addArchivedSection() {
	var err error
	pane.archivedProjects, err = pane.repo.GetAllArchived()
	if err != nil || len(pane.archivedProjects) == 0 {
		return
	}
	pane.list.AddItem("", "", 0, nil)
	pane.addSection("Archived")
	for i := range pane.archivedProjects {
		pane.addArchivedProjectToList(i)
	}
}

func (pane *ProjectPane) addArchivedProjectToList(i int) {
	idx := pane.list.GetItemCount()
	pane.selectable[idx] = true
	pane.list.AddItem("[::d]~ "+tview.Escape(pane.archivedProjects[i].Title), "", 0, func(idx int) func() {
		return func() { pane.activateArchivedProject(idx) }
	}(i))
}

func (pane *ProjectPane) activateArchivedProject(idx int) {
	pane.activeProject = &pane.archivedProjects[idx]
	taskPane.LoadProjectTasks(*pane.activeProject)
	removeThirdCol()
	projectDetailPane.SetProject(pane.activeProject)
	contents.AddItem(projectDetailPane, 25, 0, false)
	app.SetFocus(taskPane)
}

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
)

// ProjectDetailPane Displays relevant actions of current project
type ProjectDetailPane struct {
	*tview.Flex
	project    *model.Project
	archiveBtn *tview.Button
}

// NewProjectDetailPane Initializes ProjectDetailPane
func NewProjectDetailPane() *ProjectDetailPane {
	pane := ProjectDetailPane{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		archiveBtn: makeButton("[::u]A[::-]rchive", nil),
	}
	deleteBtn := makeButton("[::u]D[::-]elete Project", projectPane.RemoveActivateProject)
	clearBtn := makeButton("[::u]C[::-]lear Completed Tasks", taskPane.ClearCompletedTasks)

	deleteBtn.SetBackgroundColor(tcell.ColorRed)
	pane.archiveBtn.SetBackgroundColor(tcell.ColorDarkOliveGreen)

	pane.
		AddItem(pane.archiveBtn, 3, 1, false).
		AddItem(blankCell, 1, 1, false).
		AddItem(deleteBtn, 3, 1, false).
		AddItem(blankCell, 1, 1, false).
		AddItem(clearBtn, 3, 1, false).
		AddItem(blankCell, 0, 1, false)

	pane.SetBorder(true).SetTitle("[::u]A[::-]ctions")

	return &pane
}

// SetProject Sets the active Project
func (pd *ProjectDetailPane) SetProject(project *model.Project) {
	pd.project = project
	pd.SetTitle("[::b]" + pd.project.Title)

	if project.Archived {
		pd.archiveBtn.SetLabel("[::u]U[::-]narchive")
		pd.archiveBtn.SetSelectedFunc(projectPane.UnarchiveActiveProject)
	} else {
		pd.archiveBtn.SetLabel("[::u]A[::-]rchive")
		pd.archiveBtn.SetSelectedFunc(projectPane.ArchiveActiveProject)
	}
}

func (pd *ProjectDetailPane) isShowing() bool {
	return taskPane.activeTask == nil && projectPane.activeProject != nil
}

func (pd *ProjectDetailPane) handleShortcuts(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'a':
		if pd.project != nil && pd.project.Archived {
			projectPane.UnarchiveActiveProject()
		} else {
			projectPane.ArchiveActiveProject()
		}
		return nil
	case 'd':
		projectPane.RemoveActivateProject()
		return nil
	case 'c':
		taskPane.ClearCompletedTasks()
		return nil
	}

	return event
}

package main

import (
	"fmt"
	"os"

	"github.com/asdine/storm/v3"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
	repo "github.com/ajaxray/geek-life/repository/storm"
	"github.com/ajaxray/geek-life/util"
)

var (
	app              *tview.Application
	layout, contents *tview.Flex
	rootPages        *tview.Pages

	statusBar         *StatusBar
	projectPane       *ProjectPane
	taskPane          *TaskPane
	taskDetailPane    *TaskDetailPane
	projectDetailPane *ProjectDetailPane
	commandPalette    *CommandPalette
	terminalColumns   = 100
	terminalRows      = 40

	db          *storm.DB
	projectRepo repository.ProjectRepository
	taskRepo    repository.TaskRepository
)

const appVersion = "0.2.2"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Println("geek-life", appVersion)
			return 0
		case "help", "--help", "-h":
			printCLIHelp()
			return 0
		case "doctor":
			fmt.Printf("Geek-life %s terminal diagnostics\n%s\n", appVersion, imageProtocolDiagnostic())
			return 0
		case "backup", "migrate":
			// These commands need the repository and are handled below.
		default:
			fmt.Fprintln(os.Stderr, "Unknown command:", os.Args[1])
			printCLIHelp()
			return 2
		}
	}

	db = util.ConnectStorm()
	defer func() {
		if err := db.Close(); err != nil {
			util.LogIfError(err, "Error in closing storm Db")
		}
	}()

	projectRepo = repo.NewProjectRepository(db)
	taskRepo = repo.NewTaskRepository(db)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			if err := migrate(db); err != nil {
				fmt.Fprintln(os.Stderr, "Migration failed:", err)
				return 1
			}
			fmt.Println("Migration completed. Start geek-life normally.")
			return 0
		case "backup":
			path := defaultBackupPath()
			if len(os.Args) > 2 {
				path = os.Args[2]
			}
			if err := writeBackup(path, projectRepo, taskRepo); err != nil {
				fmt.Fprintln(os.Stderr, "Backup failed:", err)
				return 1
			}
			fmt.Println("Backup written:", path)
			return 0
		}
	}

	app = tview.NewApplication()
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		terminalColumns, terminalRows = screen.Size()
		return false
	})
	prepareStatusBar(app)
	content := prepareContentPages()
	layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(makeTitleBar(), 2, 1, false).
		AddItem(content, 0, 2, true).
		AddItem(statusBar, 1, 1, false)
	rootPages = tview.NewPages().AddPage("main", layout, true, true)
	commandPalette = NewCommandPalette()
	setKeyboardShortcuts()

	if err := app.SetRoot(rootPages, true).EnableMouse(true).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func printCLIHelp() {
	fmt.Printf("Geek-life %s\n\nUsage:\n  geek-life              Start the terminal UI\n  geek-life backup [file] Write a JSON backup\n  geek-life migrate       Rebuild database indexes\n  geek-life doctor        Show terminal/image backend diagnostics\n  geek-life version       Print version\n", appVersion)
}

func migrate(database *storm.DB) error {
	projects := repo.NewProjectRepository(database)
	activeProjects, err := projects.GetAll()
	if err != nil {
		return fmt.Errorf("load projects: %w", err)
	}
	archivedProjects, err := projects.GetAllArchived()
	if err != nil {
		return fmt.Errorf("load archived projects: %w", err)
	}
	for i := range activeProjects {
		if activeProjects[i].UUID == "" {
			if err := projects.Update(&activeProjects[i]); err != nil {
				return fmt.Errorf("backfill project identifier: %w", err)
			}
		}
	}
	for i := range archivedProjects {
		if archivedProjects[i].UUID == "" {
			if err := projects.Update(&archivedProjects[i]); err != nil {
				return fmt.Errorf("backfill archived project identifier: %w", err)
			}
		}
	}

	tasks := repo.NewTaskRepository(database)
	activeTasks, err := tasks.GetAll()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}
	deletedTasks, err := tasks.GetDeleted()
	if err != nil {
		return fmt.Errorf("load deleted tasks: %w", err)
	}
	for i := range activeTasks {
		if activeTasks[i].UUID == "" {
			if err := tasks.Update(&activeTasks[i]); err != nil {
				return fmt.Errorf("backfill task identifier: %w", err)
			}
		}
	}
	for i := range deletedTasks {
		if deletedTasks[i].UUID == "" {
			if err := tasks.Update(&deletedTasks[i]); err != nil {
				return fmt.Errorf("backfill deleted task identifier: %w", err)
			}
		}
	}

	if err := database.ReIndex(&model.Project{}); err != nil && err != storm.ErrNotFound {
		return fmt.Errorf("reindex projects: %w", err)
	}
	if err := database.ReIndex(&model.Task{}); err != nil && err != storm.ErrNotFound {
		return fmt.Errorf("reindex tasks: %w", err)
	}
	return nil
}

func setKeyboardShortcuts() *tview.Application {
	return app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if rootPages != nil && rootPages.HasPage(activeOverlayPage) {
			return event
		}
		if ignoreKeyEvt() {
			return event
		}
		switch event.Key() {
		case tcell.KeyCtrlP:
			commandPalette.Show()
			return nil
		case tcell.KeyCtrlN:
			taskPane.StartNewTask()
			return nil
		}

		// Global shortcuts
		switch event.Rune() {
		case ':':
			commandPalette.Show()
			return nil
		case '/':
			taskPane.ShowSearch()
			return nil
		case '?':
			showKeyboardHelp()
			return nil
		case 'p':
			app.SetFocus(projectPane)
			return nil
		case 't':
			app.SetFocus(taskPane)
			return nil
		}

		// Handle based on current focus. Handlers may modify event
		switch {
		case projectPane.HasFocus():
			event = projectPane.handleShortcuts(event)
		case taskPane.HasFocus():
			event = taskPane.handleShortcuts(event)
			if event != nil && projectDetailPane.isShowing() {
				event = projectDetailPane.handleShortcuts(event)
			}
		case taskDetailPane.HasFocus():
			event = taskDetailPane.handleShortcuts(event)
		}

		return event
	})
}

func prepareContentPages() *tview.Flex {
	projectPane = NewProjectPane(projectRepo)
	taskPane = NewTaskPane(projectRepo, taskRepo)
	projectDetailPane = NewProjectDetailPane()
	taskDetailPane = NewTaskDetailPane(taskRepo)

	contents = tview.NewFlex().
		AddItem(projectPane, 25, 1, true).
		AddItem(taskPane, 0, 2, false)

	return contents

}

func makeTitleBar() *tview.Flex {
	titleText := tview.NewTextView().SetText("[lime::b]Geek-life [::-]- Task Manager for geeks!").SetDynamicColors(true)
	versionInfo := tview.NewTextView().SetText("[::d]Version: " + appVersion + "   : commands   / search   ? help").SetTextAlign(tview.AlignRight).SetDynamicColors(true)

	return tview.NewFlex().
		AddItem(titleText, 0, 2, false).
		AddItem(versionInfo, 0, 1, false)
}

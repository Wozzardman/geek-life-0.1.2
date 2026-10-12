package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
	"github.com/ajaxray/geek-life/util"
)

type backupFile struct {
	Version    string          `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Projects   []model.Project `json:"projects"`
	Tasks      []model.Task    `json:"tasks"`
}

func writeBackup(path string, projects repository.ProjectRepository, tasks repository.TaskRepository) error {
	activeProjects, err := projects.GetAll()
	if err != nil {
		return err
	}
	archivedProjects, err := projects.GetAllArchived()
	if err != nil {
		return err
	}
	activeTasks, err := tasks.GetAll()
	if err != nil {
		return err
	}
	deletedTasks, err := tasks.GetDeleted()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	allProjects := make([]model.Project, 0, len(activeProjects)+len(archivedProjects))
	allProjects = append(allProjects, activeProjects...)
	allProjects = append(allProjects, archivedProjects...)
	allTasks := make([]model.Task, 0, len(activeTasks)+len(deletedTasks))
	allTasks = append(allTasks, activeTasks...)
	allTasks = append(allTasks, deletedTasks...)
	encodeErr := encoder.Encode(backupFile{
		Version:    appVersion,
		ExportedAt: time.Now(),
		Projects:   allProjects,
		Tasks:      allTasks,
	})
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func defaultBackupPath() string {
	name := fmt.Sprintf("geek-life-%s.json", time.Now().Format("20060102-150405"))
	return filepath.Join(util.DataDir(), "backups", name)
}

func backupNow() {
	path := defaultBackupPath()
	if err := writeBackup(path, projectRepo, taskRepo); err != nil {
		statusBar.showForSeconds("[red]Backup failed: "+err.Error(), 7)
		return
	}
	statusBar.showForSeconds("[lime]Backup written: "+path, 8)
}

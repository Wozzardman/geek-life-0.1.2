package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	repostorm "github.com/ajaxray/geek-life/repository/storm"
	stormdb "github.com/asdine/storm/v3"
)

func TestWriteBackupIncludesArchivedAndDeletedData(t *testing.T) {
	directory, err := ioutil.TempDir("", "geek-life-backup-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)

	database, err := stormdb.Open(filepath.Join(directory, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	projects := repostorm.NewProjectRepository(database)
	tasks := repostorm.NewTaskRepository(database)
	project, err := projects.Create("Archived project", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := projects.UpdateField(&project, "Archived", true); err != nil {
		t.Fatal(err)
	}
	task, err := tasks.Create(project, "Deleted task", "important note", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	task.DeletedAt = time.Now().Unix()
	if err := tasks.Update(&task); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(directory, "backups", "backup.json")
	if err := writeBackup(path, projects, tasks); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var backup backupFile
	if err := json.Unmarshal(data, &backup); err != nil {
		t.Fatal(err)
	}
	if backup.Version != appVersion {
		t.Fatalf("backup version = %q, want %q", backup.Version, appVersion)
	}
	if len(backup.Projects) != 1 || !backup.Projects[0].Archived {
		t.Fatalf("archived project missing from backup: %#v", backup.Projects)
	}
	if len(backup.Tasks) != 1 || backup.Tasks[0].DeletedAt == 0 {
		t.Fatalf("deleted task missing from backup: %#v", backup.Tasks)
	}
	if backup.Tasks[0].Details != "important note" || backup.Tasks[0].Title != "Deleted task" {
		t.Fatalf("task content was not preserved: %#v", backup.Tasks[0])
	}
}

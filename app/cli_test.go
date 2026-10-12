package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ajaxray/geek-life/model"
	stormdb "github.com/asdine/storm/v3"
)

func TestMigrateEmptyDatabase(t *testing.T) {
	directory, err := ioutil.TempDir("", "geek-life-migrate-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	database, err := stormdb.Open(filepath.Join(directory, "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := migrate(database); err != nil {
		t.Fatalf("migrating an empty database should succeed: %v", err)
	}
}

func TestMigrateBackfillsLegacyIdentifiers(t *testing.T) {
	directory, err := ioutil.TempDir("", "geek-life-migrate-legacy-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	database, err := stormdb.Open(filepath.Join(directory, "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project := model.Project{Title: "Legacy project"}
	if err := database.Save(&project); err != nil {
		t.Fatal(err)
	}
	task := model.Task{ProjectID: project.ID, Title: "Legacy task"}
	if err := database.Save(&task); err != nil {
		t.Fatal(err)
	}
	if err := migrate(database); err != nil {
		t.Fatal(err)
	}
	var migratedProject model.Project
	if err := database.One("ID", project.ID, &migratedProject); err != nil {
		t.Fatal(err)
	}
	var migratedTask model.Task
	if err := database.One("ID", task.ID, &migratedTask); err != nil {
		t.Fatal(err)
	}
	if migratedProject.UUID == "" || migratedTask.UUID == "" {
		t.Fatalf("migration did not backfill UUIDs: project=%q task=%q", migratedProject.UUID, migratedTask.UUID)
	}
	if migratedTask.CreatedAt == 0 || migratedTask.UpdatedAt == 0 {
		t.Fatalf("migration did not backfill task timestamps: %#v", migratedTask)
	}
}

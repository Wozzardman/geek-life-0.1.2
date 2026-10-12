package storm

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	stormdb "github.com/asdine/storm/v3"
)

func openRepositoryTestDB(t *testing.T) (*stormdb.DB, func()) {
	t.Helper()
	directory, err := ioutil.TempDir("", "geek-life-repository-test-")
	if err != nil {
		t.Fatal(err)
	}
	database, err := stormdb.Open(filepath.Join(directory, "test.db"))
	if err != nil {
		_ = os.RemoveAll(directory)
		t.Fatal(err)
	}
	return database, func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
		if err := os.RemoveAll(directory); err != nil {
			t.Errorf("remove temp database: %v", err)
		}
	}
}

func TestRepositoriesLifecycleAndQueries(t *testing.T) {
	database, cleanup := openRepositoryTestDB(t)
	defer cleanup()
	projects := NewProjectRepository(database)
	tasks := NewTaskRepository(database)

	project, err := projects.Create("Terminal", "")
	if err != nil {
		t.Fatal(err)
	}
	secondProject, err := projects.Create("Personal", "")
	if err != nil {
		t.Fatal(err)
	}
	if project.UUID == "" || project.UUID == secondProject.UUID {
		t.Fatalf("generated project UUIDs are invalid: %q and %q", project.UUID, secondProject.UUID)
	}
	byUUID, err := projects.GetByUUID(project.UUID)
	if err != nil || byUUID.ID != project.ID {
		t.Fatalf("project UUID lookup failed: %#v, %v", byUUID, err)
	}

	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.Local)
	task, err := tasks.Create(project, "Improve viewer", "full-screen zoom", "", due.Unix())
	if err != nil {
		t.Fatal(err)
	}
	secondTask, err := tasks.Create(project, "Fix installer", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if task.UUID == "" || task.UUID == secondTask.UUID {
		t.Fatalf("generated task UUIDs are invalid: %q and %q", task.UUID, secondTask.UUID)
	}

	task.Tags = []string{"terminal", "image"}
	task.Priority = 3
	task.Pinned = true
	if err := tasks.Update(&task); err != nil {
		t.Fatal(err)
	}
	loaded, err := tasks.GetByUUID(task.UUID)
	if err != nil || loaded.ID != task.ID || loaded.Priority != 3 || !loaded.Pinned {
		t.Fatalf("task round-trip failed: %#v, %v", loaded, err)
	}

	byDate, err := tasks.GetAllByDate(due.Add(16 * time.Hour))
	if err != nil || len(byDate) != 1 || byDate[0].ID != task.ID {
		t.Fatalf("date lookup did not normalize time of day: %#v, %v", byDate, err)
	}
	byRange, err := tasks.GetAllByDateRange(due.Add(8*time.Hour), due.AddDate(0, 0, 1).Add(20*time.Hour))
	if err != nil || len(byRange) != 1 || byRange[0].ID != task.ID {
		t.Fatalf("range lookup failed: %#v, %v", byRange, err)
	}

	task.DeletedAt = time.Now().Unix()
	if err := tasks.Update(&task); err != nil {
		t.Fatal(err)
	}
	active, err := tasks.GetAll()
	if err != nil || len(active) != 1 || active[0].ID != secondTask.ID {
		t.Fatalf("soft-deleted task leaked into active tasks: %#v, %v", active, err)
	}
	deleted, err := tasks.GetDeleted()
	if err != nil || len(deleted) != 1 || deleted[0].ID != task.ID {
		t.Fatalf("trash lookup failed: %#v, %v", deleted, err)
	}

	if err := projects.DeleteWithTasks(&project); err != nil {
		t.Fatal(err)
	}
	if _, err := projects.GetByID(project.ID); err != stormdb.ErrNotFound {
		t.Fatalf("deleted project lookup error = %v, want ErrNotFound", err)
	}
	if _, err := tasks.GetByID(secondTask.ID); err != stormdb.ErrNotFound {
		t.Fatalf("cascade-deleted task lookup error = %v, want ErrNotFound", err)
	}
}

func TestProjectArchiveLists(t *testing.T) {
	database, cleanup := openRepositoryTestDB(t)
	defer cleanup()
	projects := NewProjectRepository(database)

	project, err := projects.Create("Archive me", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := projects.UpdateField(&project, "Archived", true); err != nil {
		t.Fatal(err)
	}
	active, err := projects.GetAll()
	if err != nil || len(active) != 0 {
		t.Fatalf("archived project appears active: %#v, %v", active, err)
	}
	archived, err := projects.GetAllArchived()
	if err != nil || len(archived) != 1 || archived[0].ID != project.ID {
		t.Fatalf("archive lookup failed: %#v, %v", archived, err)
	}
}

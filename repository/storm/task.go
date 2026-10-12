package storm

import (
	"sort"
	"time"

	"github.com/asdine/storm/v3"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

type taskRepository struct {
	DB *storm.DB
}

// NewTaskRepository will create an object that represent the repository.Task interface
func NewTaskRepository(db *storm.DB) repository.TaskRepository {
	return &taskRepository{db}
}

func (t *taskRepository) GetAll() ([]model.Task, error) {
	all, err := t.getAllStored()
	if err != nil {
		return nil, err
	}
	active := make([]model.Task, 0, len(all))
	for _, task := range all {
		if task.DeletedAt == 0 {
			active = append(active, task)
		}
	}
	sortTasks(active)
	return active, nil
}

func (t *taskRepository) GetDeleted() ([]model.Task, error) {
	all, err := t.getAllStored()
	if err != nil {
		return nil, err
	}
	deleted := make([]model.Task, 0)
	for _, task := range all {
		if task.DeletedAt != 0 {
			deleted = append(deleted, task)
		}
	}
	sort.SliceStable(deleted, func(i, j int) bool { return deleted[i].DeletedAt > deleted[j].DeletedAt })
	return deleted, nil
}

func (t *taskRepository) GetAllByProject(project model.Project) ([]model.Task, error) {
	var tasks []model.Task
	err := t.DB.Find("ProjectID", project.ID, &tasks)
	if err == storm.ErrNotFound {
		return []model.Task{}, nil
	}
	if err != nil {
		return tasks, err
	}
	active := tasks[:0]
	for _, task := range tasks {
		if task.DeletedAt == 0 {
			active = append(active, task)
		}
	}
	tasks = active
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Order < tasks[j].Order })
	return tasks, nil
}

func (t *taskRepository) GetAllByDate(date time.Time) ([]model.Task, error) {
	var tasks []model.Task

	allTasks, err := t.GetAll()
	if err != nil {
		return nil, err
	}
	dueDate := getRoundedDueDate(date)
	for _, task := range allTasks {
		if task.DueDate == dueDate {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (t *taskRepository) GetAllByDateRange(from, to time.Time) ([]model.Task, error) {
	allTasks, err := t.GetAll()
	if err != nil {
		return nil, err
	}
	fromUnix, toUnix := getRoundedDueDate(from), getRoundedDueDate(to)
	var tasks []model.Task
	for _, task := range allTasks {
		if task.DueDate == 0 || task.DueDate < fromUnix || task.DueDate > toUnix {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (t *taskRepository) GetByID(ID int64) (model.Task, error) {
	var task model.Task
	err := t.DB.One("ID", ID, &task)
	return task, err
}

func (t *taskRepository) GetByUUID(UUID string) (model.Task, error) {
	var task model.Task
	err := t.DB.One("UUID", UUID, &task)
	return task, err
}

func (t *taskRepository) Create(project model.Project, title, details, UUID string, dueDate int64) (model.Task, error) {
	if UUID == "" {
		var err error
		UUID, err = newUUID()
		if err != nil {
			return model.Task{}, err
		}
	}
	var existing []model.Task
	_ = t.DB.Find("ProjectID", project.ID, &existing)

	now := time.Now().Unix()
	task := model.Task{
		ProjectID: project.ID,
		Title:     title,
		Details:   details,
		UUID:      UUID,
		DueDate:   dueDate,
		Order:     len(existing),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := t.DB.Save(&task)
	return task, err
}

func (t *taskRepository) Update(task *model.Task) error {
	// DB.Save instead of DB.Update so zero-value fields (empty notes, zero dates) are persisted
	if task.UUID == "" {
		UUID, err := newUUID()
		if err != nil {
			return err
		}
		task.UUID = UUID
	}
	if task.CreatedAt == 0 {
		task.CreatedAt = time.Now().Unix()
	}
	task.UpdatedAt = time.Now().Unix()
	return t.DB.Save(task)
}

func (t *taskRepository) UpdateField(task *model.Task, field string, value interface{}) error {
	if err := t.DB.UpdateField(task, field, value); err != nil {
		return err
	}
	task.UpdatedAt = time.Now().Unix()
	return t.DB.UpdateField(task, "UpdatedAt", task.UpdatedAt)
}

func (t *taskRepository) getAllStored() ([]model.Task, error) {
	var tasks []model.Task
	if err := t.DB.All(&tasks); err != nil {
		if err == storm.ErrNotFound {
			return []model.Task{}, nil
		}
		return nil, err
	}
	return tasks, nil
}

func sortTasks(tasks []model.Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Pinned != tasks[j].Pinned {
			return tasks[i].Pinned
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

func (t *taskRepository) Delete(task *model.Task) error {
	return t.DB.DeleteStruct(task)
}

func getRoundedDueDate(date time.Time) int64 {
	if date.IsZero() {
		return 0
	}
	local := date.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).Unix()
}

package storm

import (
	"sort"

	"github.com/asdine/storm/v3"

	"github.com/ajaxray/geek-life/model"
	"github.com/ajaxray/geek-life/repository"
)

type projectRepository struct {
	DB *storm.DB
}

// NewProjectRepository will create an object that represent the repository.Project interface
func NewProjectRepository(db *storm.DB) repository.ProjectRepository {
	return &projectRepository{db}
}

func (repo *projectRepository) GetAll() ([]model.Project, error) {
	var all []model.Project
	if err := repo.DB.All(&all); err != nil {
		if err == storm.ErrNotFound {
			return []model.Project{}, nil
		}
		return nil, err
	}
	var projects []model.Project
	for _, p := range all {
		if !p.Archived {
			projects = append(projects, p)
		}
	}
	sort.SliceStable(projects, func(i, j int) bool { return projects[i].Title < projects[j].Title })
	return projects, nil
}

func (repo *projectRepository) GetAllArchived() ([]model.Project, error) {
	var all []model.Project
	if err := repo.DB.All(&all); err != nil {
		if err == storm.ErrNotFound {
			return []model.Project{}, nil
		}
		return nil, err
	}
	var archived []model.Project
	for _, p := range all {
		if p.Archived {
			archived = append(archived, p)
		}
	}
	sort.SliceStable(archived, func(i, j int) bool { return archived[i].Title < archived[j].Title })
	return archived, nil
}

func (repo *projectRepository) GetByID(id int64) (model.Project, error) {
	return repo.getOneByField("ID", id)
}

func (repo *projectRepository) GetByTitle(title string) (model.Project, error) {
	return repo.getOneByField("Title", title)
}

func (repo *projectRepository) GetByUUID(UUID string) (model.Project, error) {
	return repo.getOneByField("UUID", UUID)
}

func (repo *projectRepository) Create(title, UUID string) (model.Project, error) {
	if UUID == "" {
		var err error
		UUID, err = newUUID()
		if err != nil {
			return model.Project{}, err
		}
	}
	project := model.Project{
		Title: title,
		UUID:  UUID,
	}

	err := repo.DB.Save(&project)
	return project, err
}

func (repo *projectRepository) Update(project *model.Project) error {
	if project.UUID == "" {
		UUID, err := newUUID()
		if err != nil {
			return err
		}
		project.UUID = UUID
	}
	return repo.DB.Save(project)
}

func (repo *projectRepository) Delete(project *model.Project) error {
	return repo.DB.DeleteStruct(project)
}

func (repo *projectRepository) DeleteWithTasks(project *model.Project) error {
	tx, err := repo.DB.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tasks []model.Task
	if err := tx.Find("ProjectID", project.ID, &tasks); err != nil && err != storm.ErrNotFound {
		return err
	}
	for i := range tasks {
		if err := tx.DeleteStruct(&tasks[i]); err != nil {
			return err
		}
	}
	if err := tx.DeleteStruct(project); err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *projectRepository) UpdateField(task *model.Project, field string, value interface{}) error {
	return repo.DB.UpdateField(task, field, value)
}

func (repo *projectRepository) getOneByField(fieldName string, val interface{}) (model.Project, error) {
	var project model.Project
	err := repo.DB.One(fieldName, val, &project)

	return project, err
}

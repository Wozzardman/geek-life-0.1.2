package model

// Task represent a task - the building block of the TaskManager app
type Task struct {
	ID              int64    `storm:"id,increment" json:"id"`
	ProjectID       int64    `storm:"index" json:"project_id"`
	UUID            string   `storm:"unique" json:"uuid,omitempty"`
	Title           string   `json:"text"`
	Details         string   `json:"notes"`
	Images          []string `json:"images,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Priority        int      `json:"priority,omitempty"`
	Pinned          bool     `json:"pinned,omitempty"`
	Recurrence      string   `json:"recurrence,omitempty"`
	Completed       bool     `storm:"index" json:"completed"`
	DueDate         int64    `storm:"index" json:"due_date,omitempty"`
	LastCompletedAt int64    `json:"last_completed_at,omitempty"`
	CompletedAt     int64    `json:"completed_at,omitempty"`
	DeletedAt       int64    `json:"deleted_at,omitempty"`
	CreatedAt       int64    `json:"created_at,omitempty"`
	UpdatedAt       int64    `json:"updated_at,omitempty"`
	Order           int      `json:"order"`
}

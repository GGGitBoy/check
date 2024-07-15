package apis

type Task struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Cron       string `json:"cron"`
	State      string `json:"state"`
	Rating     string `json:"rating"`
	ReportID   string `json:"report_id"`
	TemplateID string `json:"template_id"`
	NotifyID   string `json:"notify_id"`
	TaskID     string `json:"task_id"`
	Mode       string `json:"mode"`
	ErrMessage string `json:"err_message"`
}

func NewTasks() []*Task {
	return []*Task{}
}

func NewTask() *Task {
	return &Task{}
}

package agent

type Store interface {
	GetMyJobs() ([]*CronJob, error)
	GetJobById(int64) (*CronJob, error)
	StoreTaskStatus(s *TaskStatus) bool
	UpdateTaskStatus(s *TaskStatus) bool
}

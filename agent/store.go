package agent

type Store interface {
	GetMyJobs() ([]*CronJob, error)
}

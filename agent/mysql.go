package agent

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/ngaut/logging"
	"os"
)

const (
	GetMyJobsSql = "select j.id,j.name,j.create_user,j.executor,j.executor_flags,j.Signature,j.runner,j.timeout,j.timeout_trigger,j.disabled,j.schedule,j.hook,j.msg_filter,j.create_at from dcms_agent2job as a join dcms_cronjob as j on a.job_id=j.id where a.host=?"
)

type MySQLStore struct {
	//root:root@(localhost:3306)/dcms?parseTime=true&loc=Local&charset=utf8
	DSN string
}

func (m *MySQLStore) GetMyJobs() ([]*CronJob, error) {
	var host string
	cj := make([]*CronJob, 0)
	db, err := sql.Open("mysql", m.DSN)
	if err != nil {
		log.Warning("open mysql failed:", err)
		return cj, err
	}
	err = db.Ping()
	if err != nil {
		log.Warning("ping mysql handler failed:", err)
		return cj, err
	}
	if host, err = os.Hostname(); err != nil {
		log.Warning("get my hostname failed:", err)
		return cj, err
	}
	log.Debug("get myhostname is:", host)

	rows, _ := db.Query(GetMyJobsSql, host)
	for rows.Next() {
		var id int64
		var name string
		var create_user string
		var executor string
		var executor_flags string
		var sig string
		var runner string
		var timeout int64
		var timeout_trigger int
		var disabled int
		var schedule string
		var hook string
		var msg_filter string
		var create_at int64
		if err := rows.Scan(&id, &name, &create_user, &executor, &executor_flags, &sig, &runner, &timeout, &timeout_trigger, &disabled, &schedule, &hook, &msg_filter, &create_at); err != nil {
			log.Warning("rows scan failed: ", err)
		} else {
			log.Info(id, name, create_user, executor, executor_flags, runner, timeout, timeout_trigger, disabled, schedule, hook, msg_filter, create_at)
			j := &CronJob{
				Id:               id,
				Name:             name,
				CreateUser:       create_user,
				Executor:         executor,
				ExecutorFlags:    executor_flags,
				Signature:        sig,
				Runner:           runner,
				Timeout:          timeout,
				OnTimeoutTrigger: timeout_trigger,
				Schedule:         schedule,
				WebHookUrl:       hook,
				MsgFilter:        msg_filter,
				CreateAt:         create_at,
			}
			if disabled == 1 {
				j.Disabled = true
			}
			cj = append(cj, j)
		}
	}
	db.Close()
	return cj, nil
}

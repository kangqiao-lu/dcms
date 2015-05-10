package agent

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/ngaut/logging"
	"os"
)

const (
	GetMyJobsSql       = "select j.id,j.name,j.create_user,j.executor,j.executor_flags,j.signature,j.runner,j.timeout,j.timeout_trigger,j.disabled,j.schedule,j.hook,j.msg_filter,j.create_at from dcms_agent2job as a join dcms_cronjob as j on a.job_id=j.id where a.host=?"
	GetJobByIdSql      = "select j.id,j.name,j.create_user,j.executor,j.executor_flags,j.signature,j.runner,j.timeout,j.timeout_trigger,j.disabled,j.schedule,j.hook,j.msg_filter,j.create_at from dcms_agent2job as a join dcms_cronjob as j on a.job_id=j.id where a.host=? and j.id=? limit 1"
	StoreTaskSql       = "insert into dcms_crontask_log (job_id,task_id,host,status,exec_at, exec_duration,log_filename,message) values (?,?,?,?,?,?,?,?)"
	UpdateTaskSql      = "update dcms_crontask_log set status=?, exec_duration=?,message=? where task_id=? and host=?"
	CheckTaskExistsSql = "select 1 from dcms_crontask_log where task_id=? and host=?"
)

type MySQLStore struct {
	//root:root@(localhost:3306)/dcms?parseTime=true&loc=Local&charset=utf8
	DSN string
}

func (m *MySQLStore) GetJobById(id int64) (*CronJob, error) {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("GetJobById  fatal:", e)
		}
	}()
	var host string
	var j *CronJob

	db, err := sql.Open("mysql", m.DSN)
	if err != nil {
		log.Warning("open mysql failed:", err)
		return nil, err
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Warning("ping mysql handler failed:", err)
		return nil, err
	}
	if host, err = os.Hostname(); err != nil {
		log.Warning("get my hostname failed:", err)
		return nil, err
	}
	log.Debug("get myhostname is:", host)

	if rows, err := db.Query(GetJobByIdSql, host, id); err != nil {
		return nil, err
	} else {

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
				log.Info(id, name, create_user, executor, executor_flags, sig, runner, timeout, timeout_trigger, disabled, schedule, hook, msg_filter, create_at)
				j = &CronJob{
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
				return j, nil
			}
		}
	}
	return nil, errors.New("GetJobById failed.")
}

func (m *MySQLStore) GetMyJobs() ([]*CronJob, error) {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("GetMyJobs  fatal:", e)
		}
	}()
	var host string
	cj := make([]*CronJob, 0)
	db, err := sql.Open("mysql", m.DSN)
	if err != nil {
		log.Warning("open mysql failed:", err)
		return nil, err
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Warning("ping mysql handler failed:", err)
		return nil, err
	}
	if host, err = os.Hostname(); err != nil {
		log.Warning("get my hostname failed:", err)
		return nil, err
	}
	log.Debug("get myhostname is:", host)

	if rows, err := db.Query(GetMyJobsSql, host); err != nil {
		return nil, err
	} else {
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
				log.Info(id, name, create_user, executor, executor_flags, sig, runner, timeout, timeout_trigger, disabled, schedule, hook, msg_filter, create_at)
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
	}

	return cj, nil
}

func (m *MySQLStore) StoreTaskStatus(s *TaskStatus) bool {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("StoreTaskStatus  fatal:", e, s)
		}
	}()

	var (
		host string
		err  error
	)
	if host, err = os.Hostname(); err != nil {
		log.Warning("get my hostname failed:", err)
		return false
	}
	log.Debug("get myhostname is:", host)

	db, err := sql.Open("mysql", m.DSN)
	if err != nil {
		log.Warning("open mysql failed:", err)
		return false
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Warning("ping mysql handler failed:", err)
		return false
	}
	//(job_id,task_id,host,status,exec_at, exec_duration,log_filename,message)
	stmtIns, err := db.Prepare(StoreTaskSql) // ? = placeholder
	if err != nil {
		log.Warning("StoreTaskStatus get stmtIns failed: ", err)
		return false
	}
	defer stmtIns.Close()

	_, err = stmtIns.Exec(s.TaskPtr.JobId, s.TaskPtr.TaskId, host, s.Status, s.TaskPtr.ExecAt, s.TaskPtr.ExecDuration, s.TaskPtr.LogFilename, fmt.Sprintf("%s\n%s", s.Err.Error(), s.Message))
	if err != nil {
		log.Warning("StoreTaskStatus exec stmtIns failed ", err)
		return false
	}
	return true
}

func (m *MySQLStore) UpdateTaskStatus(s *TaskStatus) bool {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("UpdateTaskStatus  fatal:", e, s)
		}
	}()
	var (
		host string
		err  error
	)
	if host, err = os.Hostname(); err != nil {
		log.Warning("get my hostname failed:", err)
		return false
	}
	log.Debug("get myhostname is:", host)

	db, err := sql.Open("mysql", m.DSN)
	if err != nil {
		log.Warning("open mysql failed:", err)
		return false
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Warning("ping mysql handler failed:", err)
		return false
	}

	rows, err := db.Query(CheckTaskExistsSql, s.TaskPtr.TaskId, host)
	if err != nil {
		log.Warning("UpdateTaskStatus get CheckTaskExistsSql failed: ", err)
		return false
	}
	defer rows.Close()

	//if !rows.Next ,we suppose task log doesn't exits
	if !rows.Next() {
		ok := m.StoreTaskStatus(s)
		if !ok {
			log.Warning("StoreTaskStatus failed ", err)
			return false
		}
		return true
	}

	//(job_id,task_id,host,status,exec_at, exec_duration,log_filename,message)
	stmtIns, err := db.Prepare(UpdateTaskSql) // ? = placeholder
	if err != nil {
		log.Warning("UpdateTaskStatus get stmtIns failed: ", err)
		return false
	}
	defer stmtIns.Close()
	// status=?, exec_duration=?,message=? where task_id=? and host=?
	_, err = stmtIns.Exec(s.Status, s.TaskPtr.ExecDuration, fmt.Sprintf("%s\n%s", s.Err.Error(), s.Message), s.TaskPtr.TaskId, host)
	if err != nil {
		log.Warning("UpdateTaskStatus exec stmtIns failed ", err)
		return false
	}
	return true
}

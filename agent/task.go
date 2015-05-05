package agent

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/dongzerun/dcms/util"
	log "github.com/ngaut/logging"
)

const layout = "Jan 2, 2006 at 3:04pm (MST)"

var (
	StatusReady   int = 0
	StatusRunning int = 1
	StatusSuccess int = 2
	StatusFailed  int = 3
	StatusTimeout int = 4
	StatusKilled  int = 5
)

type TaskStatus struct {
	TaskPtr  *Task
	Command  *exec.Cmd
	Status   int
	CreateAt int64 // status create unix timestamp
	Err      error
}

type Task struct {
	JobId        int64  `json:"job_id"`
	TaskId       string `json:"task_id"`
	Status       int    `json:"status"`
	ExecAt       int64  `json:"exec_at"`
	ExecDuration int64  `json:"exec_duration"`
	LogFilename  string `json:"log_filename"`
	Job          *CronJob
	logfile      *os.File
}

func (t *Task) IsTimeout() bool {
	log.Debugf("ExecTimeAt: %d, Duration: %d Timeout: %d", t.ExecAt, time.Now().Unix()-t.ExecAt, t.Job.Timeout)
	return t.ExecAt > 0 && time.Now().Unix()-t.ExecAt > t.Job.Timeout
	// return false
}

// must run in one goroutine
func (t *Task) Exec(agent *Agent) {
	defer func() {
		if e := recover(); e != nil {

			//todo send task status to DCMS-agent
			// log.Warningf("run task: %s jobname: failed : %s", t.TaskId, t.Job.Name, e)
			ts := &TaskStatus{
				TaskPtr:  t,
				Command:  nil,
				Status:   StatusFailed,
				CreateAt: time.Now().Unix(),
				Err:      fmt.Errorf("run task: %s jobname: failed : %s", t.TaskId, t.Job.Name, e),
			}

			errstr := fmt.Sprintf("%s", e)
			if errstr == "signal: killed" {
				ts.Status = StatusKilled
			}
			t.Job.Dcms.JobStatusChan <- ts
		}
	}()

	var ts *TaskStatus
	var err error
	// log.Info("task run Exec function in goroutine")

	t.genLogFile()
	// check file signature
	tmp_md5 := util.Md5File(t.Job.Executor)
	if t.Job.Signature != tmp_md5 {
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  nil,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("cronjob: %s executor: %s signature:%s does't match db's sig:%s", t.Job.Name, t.Job.Executor, tmp_md5, t.Job.Signature),
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	} else {
		log.Info("cronjob signature match for ", t.Job.Name, t.Job.ExecutorFlags)
	}

	var u *user.User
	u, err = user.Lookup(t.Job.Runner)
	if err != nil {
		// log.Warningf("user %s not exists, task %s quit ", err, t.TaskId)
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  nil,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("user %s not exists, task %s quit ", err, t.TaskId),
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	}

	var uid int
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		// log.Warningf("uid %s conver to int failed ", uid)
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  nil,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("uid %s conver to int failed ", uid),
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	}

	// chown log file to specific t.Job.Runner user
	if err = t.logfile.Chown(uid, uid); err != nil {
		// log.Warningf("chown logfile: %s to uid: %s failed, %s", t.logfile.Name(), u.Uid, err)
		t.logfile = nil
	}
	var cmd *exec.Cmd
	if t.Job.Executor != "" && t.Job.ExecutorFlags != "" {
		cmd = exec.Command(t.Job.Executor, t.Job.ExecutorFlags)
	} else if t.Job.Executor != "" && t.Job.ExecutorFlags == "" {
		cmd = exec.Command(t.Job.Executor)
	} else {
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  cmd,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("job %s must have Executor ", t.Job.Name),
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	}

	// 	cmd := exec.Command(command, args...)
	// cmd.SysProcAttr = &syscall.SysProcAttr{}
	// cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid)}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGUSR1
	cmd.SysProcAttr.Setsid = true
	// cmd.SysProcAttr.Noctty = true

	// w := bufio.NewWriter(t.logfile)
	// if cmd.Stderr, err = w.(io.Writer); err != nil {
	// 	cmd.Stderr = nil
	// 	cmd.Stdout = nil
	// } else {
	// 	cmd.Stdout = cmd.Stderr
	// }
	cmd.Stderr = t.logfile
	cmd.Stdout = t.logfile

	if err = cmd.Start(); err != nil {
		// log.Warningf("taskid:%s cmd Start failed: %s", t.TaskId, err)
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  cmd,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("taskid:%s cmd Start failed: %s", t.TaskId, err),
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	}

	ts = &TaskStatus{
		TaskPtr:  t,
		Command:  cmd,
		Status:   StatusRunning,
		CreateAt: time.Now().Unix(),
		Err:      nil,
	}
	t.Job.Dcms.JobStatusChan <- ts
	// send cmd.process to dcms-agent

	if err = cmd.Wait(); err != nil {
		// log.Warningf("taskid:%s cmd Wait failed: %s", t.TaskId, err)
		ts = &TaskStatus{
			TaskPtr:  t,
			Command:  cmd,
			Status:   StatusFailed,
			CreateAt: time.Now().Unix(),
			Err:      fmt.Errorf("taskid:%s cmd Wait failed: %s", t.TaskId, err),
		}
		errstr := fmt.Sprintf("%s", err.Error())
		if errstr == "signal: killed" {
			ts.Status = StatusKilled
		}
		t.Job.Dcms.JobStatusChan <- ts
		return
	}
	// log.Warning("task run DONE")
	ts = &TaskStatus{
		TaskPtr:  t,
		Command:  cmd,
		Status:   StatusSuccess,
		CreateAt: time.Now().Unix(),
		Err:      nil,
	}
	t.Job.Dcms.JobStatusChan <- ts
	return
}

func (t *Task) genLogFile() {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("genLogFile fatal:", e)
		}
	}()
	d := time.Now().Format("20060102")
	filename := fmt.Sprintf("%s/DCMS-%s/%d-%s-%s.log",
		t.Job.Dcms.Conf.WorkDir,
		d,
		t.Job.Id,
		t.Job.Name,
		t.TaskId)
	log.Info("generate logfile :", filename)

	logdir := fmt.Sprintf("%s/DCMS-%s", t.Job.Dcms.Conf.WorkDir, d)

	if err := os.MkdirAll(logdir, os.ModePerm); err != nil {
		log.Warningf("in run exec goroutine, mkdir workdir %s failed!!!! ", t.Job.Dcms.Conf.WorkDir)
	}

	if f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm); err != nil {
		log.Warning("in genLogFile os.OpenFile create failed: ", f)
		t.logfile = nil
		t.LogFilename = ""
	} else {
		t.logfile = f
		t.LogFilename = filename
	}
}

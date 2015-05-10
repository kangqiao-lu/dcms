package agent

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/dongzerun/dcms/util"
	log "github.com/ngaut/logging"
)

// process task status
// todo: send msg to queue
func (agent *Agent) HandleStatusLoop() {
	defer func() {
		if e := recover(); e != nil {
			log.Warning("HandleStatusLoop  fatal, we will reboot this goroutine", e)
			go agent.HandleStatusLoop()
		}
	}()
	for {
		select {
		case s := <-agent.JobStatusChan:
			s.TaskPtr.ExecDuration = s.CreateAt - s.TaskPtr.ExecAt
			if s.Err == nil {
				s.Err = errors.New("")
			}
			if s.Status == StatusRunning && s.Command != nil {
				agent.HandleStatusRunning(s)
			} else if s.Status == StatusSuccess && s.Command != nil {
				agent.HandleStatusSuccess(s)
			} else if s.Status == StatusTimeout {
				agent.HandleStatusTimeout(s)
			} else if s.Status == StatusKilled {
				agent.HandleStatusKilled(s)
			} else {
				agent.HandleStatusFailed(s)
			}

		case <-agent.StatusLoopQuitChan:
			goto quit
		}
	}
quit:
	log.Warning("receive StatusLoopQuitChan chan, quit HandleStatusLoop")
}

func (agent *Agent) HandleStatusRunning(s *TaskStatus) {
	// panic("test")
	agent.Lock.Lock()
	agent.Process[s.TaskPtr.TaskId] = s.Command.Process
	agent.Lock.Unlock()

	s.TaskPtr.Job.LastExecAt = s.CreateAt
	s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
	s.TaskPtr.Job.LastStatus = JobRunning

	log.Debug("Task running : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name)
	if ok := agent.store.StoreTaskStatus(s); !ok {
		log.Warning("Task status Store Or Update failed ", s)
	}
}

// we will check output file, if content contain msg_filter, we will change status to Failed
func (agent *Agent) HandleStatusSuccess(s *TaskStatus) {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	if !util.HitFilter(s.TaskPtr.LogFilename, s.TaskPtr.Job.MsgFilter) {
		s.TaskPtr.Job.LastSuccessAt = s.CreateAt
		s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
		if agent.Running[s.TaskPtr.JobId].Status == StatusTimeout {
			s.TaskPtr.Job.LastStatus = JobTimeout
		} else {
			s.TaskPtr.Job.LastStatus = JobSuccess
		}
		delete(agent.Process, s.TaskPtr.TaskId)
		delete(agent.Running, s.TaskPtr.JobId)
		s.TaskPtr.Job.SuccessCnt += 1

		log.Warning("Task success : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.TaskPtr.ExecDuration)
	} else {
		s.TaskPtr.Job.LastErrAt = s.CreateAt
		s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
		s.TaskPtr.Job.LastStatus = JobFail
		s.Status = StatusFailed
		delete(agent.Process, s.TaskPtr.TaskId)
		delete(agent.Running, s.TaskPtr.JobId)
		s.TaskPtr.Job.ErrCnt += 1
		log.Warningf("Task failed : hit msg_filter error", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.TaskPtr.ExecDuration)
		s.Err = errors.New(fmt.Sprintf("Task: %s  Job: %s failed.  hit msg_filter error", s.TaskPtr.TaskId, s.TaskPtr.Job.Name))
	}
	s.Message = util.GetFileContent(s.TaskPtr.LogFilename, 65535, 1)
	if ok := agent.store.UpdateTaskStatus(s); !ok {
		log.Warning("Task status Store Or Update failed ", s)
	}
	agent.PostTaskStatus(s)
}

func (agent *Agent) HandleStatusTimeout(s *TaskStatus) {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	if s.TaskPtr.Job.OnTimeout() == TriggerKill {
		delete(agent.Process, s.TaskPtr.TaskId)
	}
	agent.Running[s.TaskPtr.JobId].Status = StatusTimeout
	s.TaskPtr.Job.LastErrAt = s.CreateAt
	s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
	s.TaskPtr.Job.TimeoutCnt += 1
	log.Warning("Task timeout : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
	s.Err = errors.New(fmt.Sprintf("Task: %s Job: %s timeout %s", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error()))
	s.Message = util.GetFileContent(s.TaskPtr.LogFilename, 65535, 1)
	if ok := agent.store.UpdateTaskStatus(s); !ok {
		log.Warning("Task status Store Or Update failed ", s)
	}
	agent.PostTaskStatus(s)
}

func (agent *Agent) HandleStatusKilled(s *TaskStatus) {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	delete(agent.Process, s.TaskPtr.TaskId)
	delete(agent.Running, s.TaskPtr.JobId)
	s.TaskPtr.Job.LastErrAt = s.CreateAt
	s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
	s.TaskPtr.Job.LastStatus = JobKilled
	s.TaskPtr.Job.ErrCnt += 1
	log.Warning("Task Killed : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
	s.Err = errors.New(fmt.Sprintf("Task: %s Job: %s killed %s", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error()))
	s.Message = util.GetFileContent(s.TaskPtr.LogFilename, 65535, 1)
	if ok := agent.store.UpdateTaskStatus(s); !ok {
		log.Warning("Task status Store Or Update failed ", s)
	}
	agent.PostTaskStatus(s)
}

func (agent *Agent) HandleStatusFailed(s *TaskStatus) {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	delete(agent.Process, s.TaskPtr.TaskId)
	delete(agent.Running, s.TaskPtr.JobId)
	s.TaskPtr.Job.LastErrAt = s.CreateAt
	s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
	s.TaskPtr.Job.LastStatus = JobFail
	s.TaskPtr.Job.ErrCnt += 1
	log.Warning("Task failed : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
	s.Err = errors.New(fmt.Sprintf("Task: %s Job: %s failed %s", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error()))
	s.Message = util.GetFileContent(s.TaskPtr.LogFilename, 65535, 1)
	log.Warning("Task GetFileContent ", s.Message)
	if ok := agent.store.UpdateTaskStatus(s); !ok {
		log.Warning("Task status Store Or Update failed ", s)
	}
	agent.PostTaskStatus(s)
}

func (agent *Agent) PostTaskStatus(s *TaskStatus) {
	if s.TaskPtr.Job.WebHookUrl == "" {
		return
	}
	host, err := os.Hostname()
	if err != nil {
		log.Warning("PostTaskStatus get my hostname failed:", err)
	}
	form := url.Values{}
	form.Add("task_id", s.TaskPtr.TaskId)
	form.Add("job_id", strconv.Itoa(int(s.TaskPtr.JobId)))
	form.Add("job_name", s.TaskPtr.Job.Name)
	form.Add("task_status", strconv.Itoa(s.Status))
	form.Add("msg", fmt.Sprintf("host:%s\n%s\n%s", host, s.Err.Error(), s.Message))
	err = util.PostUrl(s.TaskPtr.Job.WebHookUrl, form)
	if err != nil {
		log.Warning("PostTaskStatus failed for task : ", s)
	}
}

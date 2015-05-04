package agent

import (
	"fmt"
	"net/http"
	"os"
	// "os/signal"
	"sync"
	"time"
	// "strings"
	"syscall"

	"github.com/dongzerun/dcms/util"
	log "github.com/ngaut/logging"
)

type AgentConf struct {
	DBtype   string //db type : mysql or redis etc......
	MySQLdb  string //mysqldb url
	HttpPort string //http port
	WorkDir  string //work dir
	QuitTime int64  //quit timeout
}

type Agent struct {
	Lock    sync.Mutex            // mutex for thread safe
	Wg      util.WaitGroupWrapper // for management goroutine
	Conf    *AgentConf
	Jobs    map[int64]*CronJob // CronJobs belong to this agent
	Ready   map[int64]*Task    // runtime ready task, need to run
	Running map[int64]*Task    // runtime running task

	Process            map[string]*os.Process
	JobStatusChan      chan *TaskStatus
	QuitChan           chan int
	StatusLoopQuitChan chan int
	store              Store
}

func NewAgent(cfg *AgentConf) *Agent {
	agent := &Agent{
		Jobs:               make(map[int64]*CronJob, 0),
		Ready:              make(map[int64]*Task, 0),
		Running:            make(map[int64]*Task, 0),
		Process:            make(map[string]*os.Process, 0),
		QuitChan:           make(chan int, 1),
		StatusLoopQuitChan: make(chan int, 1),
		JobStatusChan:      make(chan *TaskStatus, 100),
		Conf:               cfg,
	}
	if cfg.DBtype == "mysql" {
		agent.store = &MySQLStore{
			DSN: cfg.MySQLdb,
		}
	}
	return agent
}

func (agent *Agent) GenJobs() {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	cjs, err := agent.store.GetMyJobs()
	log.Debug("GenJobs receive cjs: ", cjs)
	if err != nil {
		log.Warningf("get CronJob error: %s ", err)
		log.Warning("we will load cronjob metadata from localhost")
		//todo:
		//load cronjob file in local host
	}
	for _, cj := range cjs {
		if !cj.IsValid() {
			continue
		}
		cj.Dcms = agent
		agent.Jobs[cj.Id] = cj
		log.Debug("GenJobs receive job: ", cj)
	}
}

func (agent *Agent) TestGenJobs() {
	// job 要去重
	job := &CronJob{
		Id:         15910707764,
		Name:       "testJob",
		CreateUser: "dongzerun",
		Executor:   "/tmp/test.sh",
		Runner:     "dzsr",
		Timeout:    30,
		Disabled:   false,
		Schedule:   "*/3 * * * *",
		CreateAt:   time.Now().Unix(),
		Dcms:       agent,
	}
	log.Info(job)
	if job.IsValid() {
		agent.Jobs[job.Id] = job
	}

	job1 := &CronJob{
		Id:         15910707765,
		Name:       "testJob1",
		CreateUser: "dongzerun1",
		Executor:   "/tmp/test1.sh",
		Runner:     "dzr",
		Timeout:    30,
		Disabled:   false,
		Schedule:   "*/1 * * * *",
		CreateAt:   time.Now().Unix(),
		Dcms:       agent,
	}
	log.Info(job1)
	if job1.IsValid() {
		agent.Jobs[job1.Id] = job1
	}

	job2 := &CronJob{
		Id:               15910707769,
		Name:             "testJob2",
		CreateUser:       "dongzerun2",
		Executor:         "/tmp/test2.sh",
		Runner:           "dzr",
		Timeout:          30,
		Disabled:         false,
		Schedule:         "*/2 * * * *",
		CreateAt:         time.Now().Unix(),
		Dcms:             agent,
		OnTimeoutTrigger: TriggerKill,
	}
	log.Info(job2)
	if job2.IsValid() {
		agent.Jobs[job2.Id] = job2
	}
}

func (agent *Agent) ConsumeRunning() {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	for id, task := range agent.Running {

		// delete(agent.Running, id)
		if task.Status == StatusReady {
			log.Debug("running task: ", id, task.Job.Name)
			task.Status = StatusRunning
			task.ExecAt = time.Now().Unix()
			go task.Exec(agent)
		}
	}
}

func (agent *Agent) ConsumeReady() {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()

	for id, task := range agent.Ready {
		log.Info("consumer ready tasks: ", id, task.TaskId, task.Job.Name)
		delete(agent.Ready, id)
		task.Job.LastExecAt = time.Now().Unix()
		if _, err := agent.Running[id]; err {
			// TODO :
			//  may send message or email to job's owner
			log.Debug("ready cron aready in running queue:", id, task.Job.Name)
			continue
		}
		agent.Running[id] = task
	}
}

func (agent *Agent) CheckReady() {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()

	for id, job := range agent.Jobs {
		if _, err := agent.Ready[id]; err {
			log.Warning("cron job aready in ready queue: ", id, job.Name)
			continue
		}

		if !job.NeedSchedule() {
			continue
		}

		now := time.Now().Unix()
		task := &Task{
			JobId:  job.Id,
			TaskId: fmt.Sprintf("%d-%d", now, job.Id),
			Job:    job,
			Status: StatusReady,
			ExecAt: 0,
		}
		log.Info("add job to read task queue: ", job.Id, job.Name)
		agent.Ready[job.Id] = task
	}
}

// not thread-safe， caller must kill after get Lock
func (agent *Agent) KillTask(t *Task) {
	p, ok := agent.Process[t.TaskId]
	if !ok {
		log.Warningf("when %s timeout, can't find process in agent.Process", t.TaskId)
	} else {
		pid := p.Pid
		log.Warning("timeout pid is: we try to kill", pid)
		if err := p.Kill(); err != nil {
			// double check
			log.Warningf("kill err %s, we try again ", err)
			util.KillTaskForceByPid(pid)
		}
	}
}

func (agent *Agent) CheckTimeout() {
	log.Info("checktimeout loop for every 5 sec")
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	for _, task := range agent.Running {
		// only check running task
		if task.Status != StatusRunning {
			continue
		}
		// we will kill timeout cronjob task
		log.Info("check timeout for task:", task.TaskId, task.Job.Name)
		if task.IsTimeout() {
			if task.Job.OnTimeout() == TriggerKill {
				agent.KillTask(task)
			} else {
				log.Warning("timeout but we just ignore this :", task.TaskId)
			}
			ts := &TaskStatus{
				TaskPtr:  task,
				Command:  nil,
				Status:   StatusTimeout,
				CreateAt: time.Now().Unix(),
				Err:      fmt.Errorf("run task: %s jobname: %s timeout for %dsec", task.TaskId, task.Job.Name, time.Now().Unix()-task.ExecAt),
			}
			agent.JobStatusChan <- ts
		}
	}
}

//every 10 sec, check next run task
func (agent *Agent) TimerLoop() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Info("agent IOLoop for every 10s")
			agent.CheckReady()
			agent.ConsumeReady()
			agent.ConsumeRunning()
		case <-agent.QuitChan:
			goto quit
		}
	}
quit:
	ticker.Stop()
	log.Warning("receive quit chan, quit TimerLoop")
}

// check all task
// if task timeout, kill it or ignore
func (agent *Agent) CheckTimeoutLoop() {
	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			agent.CheckTimeout()
		case <-agent.QuitChan:
			goto quit
		}
	}
quit:
	ticker.Stop()
	log.Warning("receive quit chan, quit CheckTimeoutLoop")
}

// compare and change cronJob from store
// 1.if cronjob not exists in cjs, we suppose cronjob hase deleted
// 2.if cronjob's create_at changed ,we will fill old cj with new cj
// 3.we won't delete Disabled cronJob
func (agent *Agent) CompareAndChange(cjs []*CronJob) {
	agent.Lock.Lock()
	defer agent.Lock.Unlock()
	for _, oldcj := range agent.Jobs {
		find := false
		for _, newcj := range cjs {
			if oldcj.Id == newcj.Id {
				find = true
				break
			}
		}
		if !find {
			// we just disabled cronJob
			log.Warning("cron job disabled|removed for id: ", oldcj.Id)
			oldcj.Disabled = true
		}
	}

	for _, newcj := range cjs {
		if oldcj, ok := agent.Jobs[newcj.Id]; ok {
			//find job , compare CreateAt
			if newcj.CreateAt != oldcj.CreateAt {
				log.Warning("cron job changed for id: ", newcj.Id)
				oldcj.Name = newcj.Name
				oldcj.CreateUser = newcj.CreateUser
				oldcj.ExecutorFlags = newcj.ExecutorFlags
				oldcj.Executor = newcj.Executor
				oldcj.Runner = newcj.Runner
				oldcj.Timeout = newcj.Timeout
				oldcj.OnTimeoutTrigger = newcj.OnTimeoutTrigger
				oldcj.Disabled = newcj.Disabled
				oldcj.Schedule = newcj.Schedule
				oldcj.WebHookUrl = newcj.WebHookUrl
				oldcj.MsgFilter = newcj.MsgFilter
			}

		} else {
			// not find, just append newcj to Jobs map
			newcj.Dcms = agent
			if newcj.IsValid() {
				log.Warning("cron job Added for id: ", newcj.Id)
				agent.Jobs[newcj.Id] = newcj
			}
		}
	}
}

// every 5 min, agent get jobs from store, check if job changed
func (agent *Agent) CheckCronJobChangeLoop() {
	ticker := time.NewTicker(300 * time.Second)
	for {
		select {
		case <-ticker.C:
			cjs, err := agent.store.GetMyJobs()
			log.Info("in CheckCronJobChangeLoop got cjs:", cjs)
			if err != nil {
				log.Warning("get CronJob failed: ", err)
			} else {
				agent.CompareAndChange(cjs)
			}
		case <-agent.QuitChan:
			goto quit
		}
	}
quit:
	ticker.Stop()
	log.Warning("receive quit chan, quit CheckCronJobChangeLoop")
}

// process task status
// todo: send msg to queue
func (agent *Agent) HandleStatusLoop() {
	for {
		select {
		case s := <-agent.JobStatusChan:
			s.TaskPtr.ExecDuration = s.CreateAt - s.TaskPtr.ExecAt

			if s.Status == StatusRunning && s.Command != nil {
				agent.Lock.Lock()
				agent.Process[s.TaskPtr.TaskId] = s.Command.Process
				agent.Lock.Unlock()

				s.TaskPtr.Job.LastExecAt = s.CreateAt
				s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
				s.TaskPtr.Job.LastStatus = JobRunning

				log.Debug("Task running : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name)
			} else if s.Status == StatusSuccess && s.Command != nil {
				agent.Lock.Lock()
				s.TaskPtr.Job.LastSuccessAt = s.CreateAt
				s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
				if agent.Running[s.TaskPtr.JobId].Status == StatusTimeout {
					s.TaskPtr.Job.LastStatus = JobTimeout
				} else {
					s.TaskPtr.Job.LastStatus = JobSuccess
				}
				delete(agent.Process, s.TaskPtr.TaskId)
				delete(agent.Running, s.TaskPtr.JobId)
				agent.Lock.Unlock()

				log.Warning("Task success : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.TaskPtr.ExecDuration)
			} else if s.Status == StatusTimeout {
				agent.Lock.Lock()
				if s.TaskPtr.Job.OnTimeout() == TriggerKill {
					delete(agent.Process, s.TaskPtr.TaskId)
				}
				agent.Running[s.TaskPtr.JobId].Status = StatusTimeout
				agent.Lock.Unlock()
				s.TaskPtr.Job.LastErrAt = s.CreateAt
				s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
				log.Warning("Task timeout : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
			} else if s.Status == StatusKilled {
				agent.Lock.Lock()
				delete(agent.Process, s.TaskPtr.TaskId)
				delete(agent.Running, s.TaskPtr.JobId)
				agent.Lock.Unlock()
				s.TaskPtr.Job.LastErrAt = s.CreateAt
				s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
				s.TaskPtr.Job.LastStatus = JobKilled
				log.Warning("Task Killed : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
			} else {
				agent.Lock.Lock()
				delete(agent.Process, s.TaskPtr.TaskId)
				delete(agent.Running, s.TaskPtr.JobId)
				agent.Lock.Unlock()
				s.TaskPtr.Job.LastErrAt = s.CreateAt
				s.TaskPtr.Job.LastTaskId = s.TaskPtr.TaskId
				s.TaskPtr.Job.LastStatus = JobFail
				log.Warning("Task failed : ", s.TaskPtr.TaskId, s.TaskPtr.Job.Name, s.Err.Error())
			}

		case <-agent.StatusLoopQuitChan:
			goto quit
		}
	}
quit:
	log.Warning("receive StatusLoopQuitChan chan, quit HandleStatusLoop")
}

func (agent *Agent) Run() {
	log.Debug("agent start run .....", agent.Conf)
	// agent.TestGenJobs()
	agent.GenJobs()
	go agent.TimerLoop()
	go agent.HandleStatusLoop()
	go agent.CheckTimeoutLoop()
	go agent.CheckCronJobChangeLoop()
	go func() {
		http.ListenAndServe(":9091", nil)
	}()
	s = &Server{
		DCMS: agent,
	}
	s.Serve()
	<-agent.QuitChan
}

func (agent *Agent) Clean() {
	// we will wait for all TASK FINISHED
	// but after quit_time, we will KILL subprocess by SIGUSR1
	start_quit := time.Now().Unix()
	for l := len(agent.Process); l > 0; {
		log.Warning("process still running, we should quit after all TASK FINISHED, please wait")
		log.Warning("running task is:")
		for task, _ := range agent.Process {
			log.Warningf("%s ", task)
		}
		time.Sleep(10 * time.Second)
		l = len(agent.Process)
		if now := time.Now().Unix(); now-start_quit > agent.Conf.QuitTime {
			log.Warning("quit_time timeout, we will kill subprocess by SIGUSR1")
			for task_id, p := range agent.Process {
				if err := p.Signal(syscall.SIGUSR1); err != nil {
					log.Warningf("SIGUSR1 task:%s failed...", task_id)
				}
				log.Warningf("SIGUSR1 task:%s OK...wait subprocess quit", task_id)
			}
			goto quit
		}

	}
quit:
	time.Sleep(10 * time.Second)
	close(agent.StatusLoopQuitChan)
	log.Warning("all process DONE, we quit success.")
}

// preRun used to check work_dir, connected mysql etc...
func (agent *Agent) preRun() {
	if err := os.MkdirAll(agent.Conf.WorkDir, os.ModePerm); err != nil {
		log.Fatal("os.MkdirAll log file failed : ", err)
	}
}

package agent

import (
	"encoding/json"
	"fmt"
	"github.com/go-martini/martini"
	log "github.com/ngaut/logging"
	"strconv"
	"time"
)

var (
	s *Server
)

type Server struct {
	DCMS *Agent
}

//  "/pre/getsecbyid?sec=xxxx&preid=xxxx"
// "/pre/:preid/sec/:secid"
// "/sec/:secid?pre=preid"
func (s *Server) Serve() {
	log.Info("start Server http api")
	m := martini.Classic()
	m.Get("/", TestApi)
	m.Get("/jobs", s.GetAllJobs)
	m.Get("/jobs/:jobid", s.GetJobById)
	m.Delete("/jobs/:jobid", s.DeleteJobById)
	m.Get("/tasks", s.GetAllTasks)
	m.Get("/tasks/:taskid", s.GetTaskById)
	m.Delete("/tasks/:taskid", s.DeleteTaskById)
	m.RunOnAddr(fmt.Sprintf(":%s", s.DCMS.Conf.HttpPort))
}

func TestApi() (int, string) {
	return 201, "test ok "
}
func (s *Server) GetJobById(p martini.Params) (int, string) {
	log.Debug("Server dcms http_api GetJobById")
	jobid, ok := p["jobid"]
	if !ok {
		return responseError(500, "GetJobById without jobid")
	}
	id, err := strconv.Atoi(jobid)
	if err != nil {
		return responseError(500, fmt.Sprintf("convert string:%s to int failed", jobid))
	}
	if job, ok := s.DCMS.Jobs[int64(id)]; ok {
		return responseSuccess(job)
	}
	return responseError(404, fmt.Sprintf("Job:%s not fuound", jobid))
}

func (s *Server) GetAllJobs() (int, string) {
	log.Debug("Server dcms http_api GetAllJobs")
	cronSlice := make([]*CronJob, 0)
	for _, job := range s.DCMS.Jobs {
		cronSlice = append(cronSlice, job)
	}
	return responseSuccess(cronSlice)
}

//we suppose Caller has updated store's Job metadata
//DeleteJobById just disabled Job, and update Job's CreateAt
func (s *Server) DeleteJobById(p martini.Params) (int, string) {
	log.Debug("Server dcms http_api DeleteJobById")
	jobid, ok := p["jobid"]
	if !ok {
		return responseError(500, "GetJobById without jobid")
	}
	id, err := strconv.Atoi(jobid)
	if err != nil {
		return responseError(500, fmt.Sprintf("convert string:%s to int failed", jobid))
	}

	//different with get, DELETE must get Lock
	s.DCMS.Lock.Lock()
	defer s.DCMS.Lock.Unlock()
	if job, ok := s.DCMS.Jobs[int64(id)]; ok {
		job.Disabled = true
		job.CreateAt = time.Now().Unix()
		return responseSuccess(fmt.Sprintf("Job:%s already disabled", jobid))
	}
	return responseError(404, fmt.Sprintf("Job:%s not fuound", jobid))

}
func (s *Server) GetAllTasks() (int, string) {
	log.Debug("Server dcms http_api GetAllTasks")
	taskSlice := make([]*Task, 0)
	for _, task := range s.DCMS.Running {
		taskSlice = append(taskSlice, task)
	}
	return responseSuccess(taskSlice)
}
func (s *Server) GetTaskById(p martini.Params) (int, string) {
	log.Debug("Server dcms http_api GetJobById")
	taskid, ok := p["taskid"]
	if !ok {
		return responseError(500, "GetTaskById without taskid")
	}
	for _, task := range s.DCMS.Running {
		if task.TaskId != taskid {
			continue
		}
		return responseSuccess(task)
	}
	return responseError(404, fmt.Sprintf("Task:%s not fuound", taskid))
}

//DeleteTaskById will kill subprocess of task
func (s *Server) DeleteTaskById(p martini.Params) (int, string) {
	log.Debug("Server dcms http_api DeleteJobById")
	taskid, ok := p["taskid"]
	if !ok {
		return responseError(500, "GetTaskById without taskid")
	}

	//we will KILL subprocess async, in one goroutine
	go func(taskid string) {
		defer func() {
			if err := recover(); err != nil {
				log.Warningf("Delete Task By Id:%s panic: %s", taskid, err)
			}
		}()
		s.DCMS.Lock.Lock()
		defer s.DCMS.Lock.Unlock()
		for _, task := range s.DCMS.Running {
			if task.TaskId != taskid {
				continue
			}
			s.DCMS.KillTask(task)
			return
		}
		log.Warningf("Delete Task By Id:%s not exists or may be done", taskid)
	}(taskid)

	return responseSuccess(fmt.Sprintf("Task:%s will be killed async, or may be done normal", taskid))

}
func responseJson(statusCode int, obj interface{}) (int, string) {
	if obj != nil {
		content, _ := json.MarshalIndent(obj, " ", "  ")
		return statusCode, string(content)
	}
	return statusCode, ""
}

func responseError(ret int, msg string) (int, string) {
	return responseJson(500, map[string]interface{}{
		"ret": ret,
		"msg": msg,
	})
}

func responseSuccess(data interface{}) (int, string) {
	return responseJson(200, map[string]interface{}{
		"ret":  0,
		"data": data,
	})
}

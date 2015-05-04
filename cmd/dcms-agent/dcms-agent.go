package main

import (
	"flag"
	"os"
	"os/signal"
	// "strings"
	"syscall"
	"time"

	"github.com/dongzerun/dcms/agent"
	log "github.com/ngaut/logging"
)

var (
	dbtype    = flag.String("dbtype", "mysql", "store cron job db type")
	db        = flag.String("db", "root:root@tcp(locahost)/dcms", "mysql url used for jobs")
	port      = flag.String("port", "8001", "management port by http protocol")
	work_dir  = flag.String("work_dir", "/tmp", "work dir, used to save log file etc..")
	quit_time = flag.Int64("quit_time", 3600, "when agent recevie, we wait quit_time sec for all TASK FINISHED")
)

func main() {
	flag.Parse()
	log.Info("flag parse: ", *db, *port)

	cfg := &agent.AgentConf{
		DBtype:   *dbtype,
		MySQLdb:  *db,
		HttpPort: *port,
		WorkDir:  *work_dir,
	}
	agent := agent.NewAgent(cfg)
	quit := agent.QuitChan
	sq := agent.StatusLoopQuitChan
	go agent.Run()

	// handle quit signal, we should quit after all TASK FINISHED
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-sc
	log.Warning("main receive quit signal...")
	close(quit)

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
		if now := time.Now().Unix(); now-start_quit > *quit_time {
			log.Warning("quit_time timeout, we will kill subprocess by SIGUSR1")
			for task_id, p := range agent.Process {
				if err := p.Signal(syscall.SIGUSR1); err != nil {
					log.Warningf("SIGUSR1 task:%s failed...", task_id)
				}
				log.Warningf("SIGUSR1 task:%s OK...wait subprocess quit", task_id)
			}
			goto quit_sq
		}

	}
quit_sq:
	time.Sleep(10 * time.Second)
	close(sq)
	log.Warning("all process DONE, we quit success.")
}

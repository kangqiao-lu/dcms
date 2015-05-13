package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/dongzerun/dcms/agent"
	log "github.com/ngaut/logging"
)

var (
	dbtype    = flag.String("dbtype", "mysql", "store cron job db type")
	db        = flag.String("db", "root:root@tcp(localhost)/dcms", "mysql url used for jobs")
	port      = flag.String("port", "8001", "management port by http protocol")
	work_dir  = flag.String("work_dir", "/tmp", "work dir, used to save log file etc..")
	quit_time = flag.Int64("quit_time", 3600, "when agent recevie, we wait quit_time sec for all TASK FINISHED")
	verbose   = flag.String("verbose", "info", "log verbose:info, debug, warning, fatal")
)

func main() {
	flag.Parse()
	log.Info("flag parse: ", *db, *port)
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	LogVerbose(*verbose)

	cfg := &agent.AgentConf{
		DBtype:   *dbtype,
		MySQLdb:  *db,
		HttpPort: *port,
		WorkDir:  *work_dir,
		QuitTime: *quit_time,
	}
	agent := agent.NewAgent(cfg)

	quit := agent.QuitChan
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
	agent.Clean()
}

func LogVerbose(v string) {
	switch v {
	case "info":
		log.SetLevelByString("info")
	case "debug":
		log.SetLevelByString("debug")
	case "warning":
		log.SetLevelByString("warning")
	case "fatal":
		log.SetLevelByString("fatal")
	}
}

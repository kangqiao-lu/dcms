package agent

import (
	"github.com/go-martini/martini"
	log "github.com/ngaut/logging"
)

var (
	s *Server
)

type Server struct {
	DCMS *Agent
}

func (s *Server) Serve() {
	log.Info("start Server http api")
	m := martini.Classic()
	m.Get("/", TestApi)
	m.RunOnAddr(":9091")
}

func TestApi() (int, string) {
	return 201, "test ok "
}

package util

import (
	"crypto/md5"
	"fmt"
	log "github.com/ngaut/logging"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
)

func TouchFile(f string) error {
	return ioutil.WriteFile(f, nil, 0644)
}

func KillTaskForceByPid(pid int) {
	exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}

func Md5File(filename string) string {
	f, err := os.Open(filename)
	if err != nil {
		log.Warning("open filename failed: ", filename)
	}
	h := md5.New()
	io.Copy(h, f)
	return fmt.Sprintf("%x", h.Sum(nil))
}

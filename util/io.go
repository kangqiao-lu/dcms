package util

import (
	"bufio"
	"crypto/md5"
	"fmt"
	log "github.com/ngaut/logging"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
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

// filter must split by '|', for example "fatal|error|fail|failed"
func HitFilter(filename string, filter string) bool {
	log.Debug("HitFilter run:", filename, filter)
	filterExp, err := regexp.Compile(fmt.Sprintf(`(?i:(%s))`, filter))
	if err != nil {
		log.Warningf("HitFilter regexp.Compile for %s failed:%s", filter, err)
		return false
	}

	if f, err := os.Open(filename); err != nil {
		log.Warning("HitFilter open file failed ", filename, err)
		return false
	} else {
		defer f.Close()
		freader := bufio.NewReader(f)
		for {
			var str string
			str, err = freader.ReadString('\n')
			s := filterExp.FindStringSubmatch(str)
			if len(s) > 0 {
				log.Debugf("HitFilter hit msg_filter ", s, str)
				return true
			}
			if err == io.EOF {
				break
			}
		}
	}
	return false
}

// direction 0 head, 1 tail
// we only return retain-size string to Caller
func GetFileContent(filename string, retain int64, direction int) string {
	log.Debugf("GetFileContent ", filename, retain)
	f, err := os.Open(filename)
	if err != nil {
		log.Warning("GetOutPut open failed ", filename, err)
		return ""
	}
	defer f.Close()

	fs, err := f.Stat()
	if err != nil {
		log.Warning("GetOutPut get Stat failed ", filename, err)
		return ""
	}
	var buf []byte
	seek_at := int64(0)
	if fs.Size() > retain && direction == 1 {
		seek_at = fs.Size() - retain
		buf = make([]byte, retain)
	}
	buf = make([]byte, fs.Size())

	f.Seek(seek_at, 0)

	if _, err := f.Read(buf); err != nil && err != io.EOF {
		log.Warning("GetOutPut read buf failed ", err)
		return ""
	}
	return string(buf)
}

func PostUrl(url string, form url.Values) error {
	_, err := http.PostForm(url, form)
	if err != nil {
		// log.Warning("post url failed ", url, form)
		return err
	}
	return nil
}

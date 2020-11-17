package dlog

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bufferSize = 256 * 1024
)

func getLastCheck(now time.Time) uint64 {
	return uint64(now.Year())*1000000 + uint64(now.Month())*10000 + uint64(now.Day())*100 + uint64(now.Hour())
}

type syncBuffer struct {
	*bufio.Writer
	file     *os.File
	count    uint64
	cur      int
	filePath string
	parent   *FileBackend
}

func (u *syncBuffer) Sync() error {
	return u.file.Sync()
}

func (u *syncBuffer) close() {
	u.Flush()
	u.Sync()
	u.file.Close()
}

func (u *syncBuffer) write(b []byte) {
	if !u.parent.rotateByHour && u.parent.maxSize > 0 && u.parent.rotateNum > 0 && u.count+uint64(len(b)) >= u.parent.maxSize {
		os.Rename(u.filePath, u.filePath+fmt.Sprintf(".%03d", u.cur))
		u.cur++
		if u.cur >= u.parent.rotateNum {
			u.cur = 0
		}
		u.count = 0
	}
	u.count += uint64(len(b))
	u.Writer.Write(b)
}

type FileBackend struct {
	mu            sync.Mutex
	dir           string //directory for log files
	logFileName   string
	files         syncBuffer
	flushInterval time.Duration
	rotateNum     int
	maxSize       uint64
	fall          bool
	rotateByHour  bool
	lastCheck     uint64
	reg           *regexp.Regexp // for rotatebyhour log del...
	keepHours     uint           // keep how many hours old, only make sense when rotatebyhour is T
}

func (u *FileBackend) Flush() {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.files.Flush()
	u.files.Sync()
}

func (u *FileBackend) close() {
	u.Flush()
}

func (u *FileBackend) flushDaemon() {
	for {
		time.Sleep(u.flushInterval)
		u.Flush()
	}
}

func shouldDel(fileName string, left uint) bool {
	// tag should be like 2016071114
	tagInt, err := strconv.Atoi(strings.Split(fileName, ".")[2])
	if err != nil {
		return false
	}

	point := time.Now().Unix() - int64(left*3600)

	if getLastCheck(time.Unix(point, 0)) > uint64(tagInt) {
		return true
	}

	return false

}

func (u *FileBackend) rotateByHourDaemon() {
	for {
		time.Sleep(time.Second * 1)

		if u.rotateByHour {
			check := getLastCheck(time.Now())
			if u.lastCheck < check {
				os.Rename(u.files.filePath, u.files.filePath+fmt.Sprintf(".%d", u.lastCheck))
				u.lastCheck = check
			}

			// also check log dir to del overtime files
			files, err := ioutil.ReadDir(u.dir)
			if err == nil {
				for _, file := range files {
					// exactly match, then we
					if file.Name() == u.reg.FindString(file.Name()) &&
						shouldDel(file.Name(), u.keepHours) {
						os.Remove(filepath.Join(u.dir, file.Name()))
					}
				}
			}
		}
	}
}

func (u *FileBackend) monitorFiles() {
	for range time.NewTicker(time.Second * 5).C {
		fileName := path.Join(u.dir, u.logFileName)
		if _, err := os.Stat(fileName); err != nil && os.IsNotExist(err) {
			if f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				u.mu.Lock()
				u.files.close()
				u.files.Writer = bufio.NewWriterSize(f, bufferSize)
				u.files.file = f
				u.mu.Unlock()
			}
		}

	}
}

func (u *FileBackend) Log(s Severity, msg []byte) {
	u.mu.Lock()
	switch s {
	case FATAL:
		u.files.write(msg)
	case ERROR:
		u.files.write(msg)
	case WARNING:
		u.files.write(msg)
	case INFO:
		u.files.write(msg)
	case DEBUG:
		u.files.write(msg)
	}
	if u.fall && s < INFO {
		u.files.write(msg)
	}
	u.mu.Unlock()
	if s == FATAL {
		u.Flush()
	}
}

func (u *FileBackend) Rotate(rotateNum1 int, maxSize1 uint64) {
	u.rotateNum = rotateNum1
	u.maxSize = maxSize1
}

func (u *FileBackend) SetRotateByHour(rotateByHour bool) {
	u.rotateByHour = rotateByHour
	if u.rotateByHour {
		u.lastCheck = getLastCheck(time.Now())
	} else {
		u.lastCheck = 0
	}
}

func (u *FileBackend) SetKeepHours(hours uint) {
	u.keepHours = hours
}

func (u *FileBackend) Fall() {
	u.fall = true
}

func (u *FileBackend) SetFlushDuration(t time.Duration) {
	if t >= time.Second {
		u.flushInterval = t
	} else {
		u.flushInterval = time.Second
	}
}

func NewFileBackend(logFilePath string, logFileName string) (*FileBackend, error) {
	if err := os.MkdirAll(logFilePath, 0755); err != nil {
		return nil, err
	}
	var fb FileBackend
	fb.dir = logFilePath

	if logFileName == "" {
		logFileName = "run.log"
	}

	if !strings.HasSuffix(logFileName, ".log") {
		logFileName = fmt.Sprintf("%s.log", logFileName)
	}

	fb.logFileName = logFileName

	fb.reg = regexp.MustCompile(fmt.Sprintf("%s\\.log\\.20[0-9]{8}", logFileName))

	fileName := path.Join(logFilePath, logFileName)
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	fb.files = syncBuffer{Writer: bufio.NewWriterSize(f, bufferSize), file: f, filePath: fileName, parent: &fb}

	// default
	fb.flushInterval = time.Second * 3
	fb.rotateNum = 20
	fb.maxSize = 1024 * 1024 * 1024
	fb.rotateByHour = false
	fb.lastCheck = 0
	// init reg to match files
	// ONLY cover this centry...
	fb.reg = regexp.MustCompile(fmt.Sprintf("%s\\.20[0-9]{8}", logFileName))
	fb.keepHours = 24 * 7

	go fb.flushDaemon()
	go fb.monitorFiles()
	go fb.rotateByHourDaemon()
	return &fb, nil
}

func Rotate(rotateNum1 int, maxSize1 uint64) {
	if fileback != nil {
		fileback.Rotate(rotateNum1, maxSize1)
	}
}

func Fall() {
	if fileback != nil {
		fileback.Fall()
	}
}

func SetFlushDuration(t time.Duration) {
	if fileback != nil {
		fileback.SetFlushDuration(t)
	}
}

func SetRotateByHour(rotateByHour bool) {
	if fileback != nil {
		fileback.SetRotateByHour(rotateByHour)
	}
}

func SetKeepHours(hours uint) {
	if fileback != nil {
		fileback.SetKeepHours(hours)
	}
}

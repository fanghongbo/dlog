// +build linux darwin freebsd openbsd solaris

package dlog

import (
	"fmt"
	"log/syslog"
	"os"
)

const numSeverity = 5

type syslogBackend struct {
	writer [numSeverity]*syslog.Writer
	buf    [numSeverity]chan []byte
}

var SyslogPriorityMap = map[string]syslog.Priority{
	"local0": syslog.LOG_LOCAL0,
	"local1": syslog.LOG_LOCAL1,
	"local2": syslog.LOG_LOCAL2,
	"local3": syslog.LOG_LOCAL3,
	"local4": syslog.LOG_LOCAL4,
	"local5": syslog.LOG_LOCAL5,
	"local6": syslog.LOG_LOCAL6,
	"local7": syslog.LOG_LOCAL7,
}

var pmap = []syslog.Priority{syslog.LOG_EMERG, syslog.LOG_ERR, syslog.LOG_WARNING, syslog.LOG_INFO, syslog.LOG_DEBUG}

func NewSyslogBackend(priorityStr string, tag string) (*syslogBackend, error) {
	priority, ok := SyslogPriorityMap[priorityStr]
	if !ok {
		return nil, fmt.Errorf("unknown syslog priority: %s", priorityStr)
	}
	var err error
	var b syslogBackend
	for i := 0; i < numSeverity; i++ {
		b.writer[i], err = syslog.New(priority|pmap[i], tag)
		if err != nil {
			return nil, err
		}
		b.buf[i] = make(chan []byte, 1<<16)
	}
	b.log()
	return &b, nil
}

func DialSyslogBackend(network, raddr string, priority syslog.Priority, tag string) (*syslogBackend, error) {
	var err error
	var b syslogBackend
	for i := 0; i < numSeverity; i++ {
		b.writer[i], err = syslog.Dial(network, raddr, priority|pmap[i], tag+severityName[i])
		if err != nil {
			return nil, err
		}
		b.buf[i] = make(chan []byte, 1<<16)
	}
	b.log()
	return &b, nil
}

func (u *syslogBackend) Log(s Severity, msg []byte) {
	msg1 := make([]byte, len(msg))
	copy(msg1, msg)
	switch s {
	case FATAL:
		u.tryPutInBuf(FATAL, msg1)
	case ERROR:
		u.tryPutInBuf(ERROR, msg1)
	case WARNING:
		u.tryPutInBuf(WARNING, msg1)
	case INFO:
		u.tryPutInBuf(INFO, msg1)
	case DEBUG:
		u.tryPutInBuf(DEBUG, msg1)
	}
}

func (u *syslogBackend) close() {
	for i := 0; i < numSeverity; i++ {
		u.writer[i].Close()
	}
}

func (u *syslogBackend) tryPutInBuf(s Severity, msg []byte) {
	select {
	case u.buf[s] <- msg:
	default:
		os.Stderr.Write(msg)
	}
}

func (u *syslogBackend) log() {
	for i := 0; i < numSeverity; i++ {
		go func(index int) {
			for {
				msg := <-u.buf[index]
				u.writer[index].Write(msg[27:])
			}
		}(i)
	}
}

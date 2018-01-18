package termlogger

import (
	"time"
)

type frame struct {
	Time  time.Time
	Type  string
	Input []byte
}

// Formatter is for Logger to store logs coming from user shell sessions
type Formatter interface {
	WriteLog(frame) error
	Close() error
}

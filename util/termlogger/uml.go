package termlogger

import (
	"encoding/binary"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// umlLogHeader is the data header for UML compatible log
type umlLogHeader struct {
	op   int32
	tty  uint32
	len  int32
	dir  TTYDirection
	sec  uint32
	usec uint32
}

// UmlLog is the instance for storing logging information, like io
type UMLHook struct {
	tty    uint32
	name   string
	stdout chan []byte
}

// TTYDirection specifies the direction of data
type TTYDirection int32

const (
	ttyLogOpen  = 1
	ttyLogClose = 2
	ttyLogWrite = 3
)

const (
	// TTYRead Indicates reading from terminal
	TTYRead TTYDirection = 1
	// TTYWrite Indicates writing to terminal
	TTYWrite TTYDirection = 2
)

// NewUMLHook creates a new logrus hook instance and will create the UML log file
func NewUMLHook(ttyID uint32, logFile string) (LogHook, error) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	t := &UMLHook{
		tty:    ttyID,
		name:   logFile,
		stdout: make(chan []byte, 100),
	}
	now := time.Now()
	header := umlLogHeader{
		op:   ttyLogOpen,
		tty:  t.tty,
		len:  0,
		dir:  0,
		sec:  uint32(now.Unix()),     //For compatibility, works till 2038
		usec: uint32(now.UnixNano()), //For compatibility, works till 2038
	}
	err = binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (uLog *UMLHook) Fire(entry *log.Entry) error {
	file, err := os.OpenFile(uLog.name, os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		return err
	}
	size := len([]byte(entry.Message))
	var ttyDir TTYDirection
	if entry.Data["dir"] == "i" {
		ttyDir = TTYRead
	} else {
		ttyDir = TTYWrite
	}
	header := umlLogHeader{
		op:   ttyLogWrite,
		tty:  uLog.tty,
		len:  int32(size),
		dir:  ttyDir,
		sec:  uint32(entry.Time.Unix()),     //For compatibility, works till 2038
		usec: uint32(entry.Time.UnixNano()), //For compatibility, works till 2038
	}
	err = binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(entry.Message))
	if err != nil {
		return err
	}
	return nil
}

// Close closes the log file for writing UML logs
func (uLog *UMLHook) Close() error {
	now := time.Now()
	file, _ := os.OpenFile(uLog.name, os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	header := umlLogHeader{
		op:   ttyLogClose,
		tty:  uLog.tty,
		len:  0,
		dir:  0,
		sec:  uint32(now.Unix()),     //For compatibility, works till 2038
		usec: uint32(now.UnixNano()), //For compatibility, works till 2038
	}
	binary.Write(file, binary.LittleEndian, header)
	return nil
}

func (uLog *UMLHook) Levels() []log.Level {
	return log.AllLevels
}

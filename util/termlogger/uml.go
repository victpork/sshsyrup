package termlogger

import (
	"encoding/binary"
	"io"
	"os"
	"time"
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
type UmlLog struct {
	tty     uint32
	name    string
	stdout  chan []byte
	logChan chan frame
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

// NewUMLLogger creates a new logger instance and will create the UML log file
func NewUMLLogger(ttyID uint32, logFile string, readWriter io.ReadWriter) Formatter {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		panic("Cannot create log file")
	}

	t := &UmlLog{
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
		panic("Could not write to log file")
	}
	return t
}

func (uLog *UmlLog) WriteLog(f frame) error {
	file, err := os.OpenFile(uLog.name, os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		return err
	}
	size := len(f.Input)
	var ttyDir TTYDirection
	if f.Type == "i" {
		ttyDir = TTYRead
	} else {
		ttyDir = TTYWrite
	}
	header := umlLogHeader{
		op:   ttyLogWrite,
		tty:  uLog.tty,
		len:  int32(size),
		dir:  ttyDir,
		sec:  uint32(f.Time.Unix()),     //For compatibility, works till 2038
		usec: uint32(f.Time.UnixNano()), //For compatibility, works till 2038
	}
	err = binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return err
	}
	_, err = file.Write(f.Input)
	if err != nil {
		return err
	}
	return nil
}

// Close closes the log file for writing UML logs
func (uLog UmlLog) Close() error {
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

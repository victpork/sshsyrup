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
	tty        uint32
	name       string
	readWriter io.ReadWriter
	stdout     chan []byte
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

func (uLog *UmlLog) Read(p []byte) (n int, err error) {
	return uLog.readWriter.Read(p)
}

func (uLog *UmlLog) Write(p []byte) (n int, err error) {
	uLog.stdout <- p
	return uLog.readWriter.Write(p)
}

// NewUMLLogger creates a new logger instance and will create the UML log file
func NewUMLLogger(ttyID uint32, logFile string, readWriter io.ReadWriter) (t UmlLog) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		panic("Cannot create log file")
	}

	t = UmlLog{
		tty:        ttyID,
		name:       logFile,
		readWriter: readWriter,
		stdout:     make(chan []byte, 100),
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

	go func(c chan []byte) {
		for data := range c {
			file, err := os.OpenFile(t.name, os.O_APPEND|os.O_WRONLY, 0666)

			if err != nil {
				panic("Cannot create log file")
			}
			size := len(data)
			now := time.Now()

			header := umlLogHeader{
				op:   ttyLogWrite,
				tty:  t.tty,
				len:  int32(size),
				dir:  TTYWrite,
				sec:  uint32(now.Unix()),     //For compatibility, works till 2038
				usec: uint32(now.UnixNano()), //For compatibility, works till 2038
			}
			err = binary.Write(file, binary.LittleEndian, header)
			if err != nil {
				return
			}
			_, err = file.Write(data)
		}
	}(t.stdout)
	return
}

// Close closes the log file for writing UML logs
func (t UmlLog) Close() (err error) {
	now := time.Now()
	file, err := os.OpenFile(t.name, os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	header := umlLogHeader{
		op:   ttyLogClose,
		tty:  t.tty,
		len:  0,
		dir:  0,
		sec:  uint32(now.Unix()),     //For compatibility, works till 2038
		usec: uint32(now.UnixNano()), //For compatibility, works till 2038
	}
	err = binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return
	}
	return nil
}

package termlogger

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type frame struct {
	Time  float64
	Type  string
	Input string
}

type asciiCast struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Command   string            `json:"command"`
	Title     string            `json:"title"`
	Env       map[string]string `json:"env"`
}

type ASCIICastLog struct {
	data       asciiCast
	fileName   string
	createTime time.Time
	readWriter io.ReadWriter
	stdout     chan []byte
}

// NewACastLogger creates a new ASCIICast logger
func NewACastLogger(width, height int, command, title, prefix string, input io.ReadWriter) (aLog *ASCIICastLog) {
	now := time.Now()
	aLog = new(ASCIICastLog)
	header := asciiCast{
		Version:   1,
		Width:     width,
		Height:    height,
		Command:   command,
		Timestamp: now.Unix(),
		Title:     title,
		Env: map[string]string{
			"TERM":  "vt100",
			"SHELL": "/bin/sh",
		},
	}
	aLog.data, aLog.readWriter, aLog.createTime = header, input, now
	aLog.fileName = "logs/" + prefix + aLog.createTime.Format("20060102-150405") + ".cast"
	b, err := json.Marshal(aLog.data)
	if err != nil {
		log.Printf("Error when marshalling log data, quitting")
		return
	}
	b = append(b, '\r', '\n')
	if err = ioutil.WriteFile(aLog.fileName, b, 0600); err != nil {
		log.Printf("Error when writing log file %v, quitting", aLog.fileName)
		return
	}
	aLog.stdout = make(chan []byte, 100)

	go func(c <-chan []byte) {
		for p := range c {
			now := time.Now()
			diff := now.Sub(aLog.createTime)
			file, err := os.OpenFile(aLog.fileName, os.O_APPEND|os.O_WRONLY, 0666)
			if err != nil {
				log.Println("Log write error")
			}
			if escStr, err := json.Marshal(string(p)); err == nil {
				file.Write([]byte(fmt.Sprintf("[%f, \"%v\", %v]\r\n", diff.Seconds(), "o", string(escStr))))
			} else {
				log.Println("Log write error")
			}
			file.Close()
		}
		// TODO: Upload cast to asciinema.org

	}(aLog.stdout)
	return
}

func (aLog *ASCIICastLog) Read(p []byte) (n int, err error) {
	return aLog.readWriter.Read(p)
}

func (aLog *ASCIICastLog) Write(p []byte) (n int, err error) {
	aLog.stdout <- p
	return aLog.readWriter.Write(p)
}

func (aLog *ASCIICastLog) Close() {
	close(aLog.stdout)
	return
}

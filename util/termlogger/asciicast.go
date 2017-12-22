package termlogger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
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

// ASCIICastLog is the logger object for storing logging settings
type ASCIICastLog struct {
	data        asciiCast
	fileName    string
	createTime  time.Time
	readWriter  io.ReadWriter
	stdout      chan []byte
	userName    string
	apikey      string
	apiEndpoint string
	elapse      time.Duration
	htClient    *http.Client
}

const (
	logTimeFormat string = "20060102-150405"
)

// NewACastLogger creates a new ASCIICast logger
func NewACastLogger(width, height int, command, title, prefix, apiEndPt, apiKey string, input io.ReadWriter) (aLog *ASCIICastLog) {
	now := time.Now()
	header := asciiCast{
		Version:   2,
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
	aLog = &ASCIICastLog{
		data:        header,
		readWriter:  input,
		createTime:  now,
		apikey:      apiKey,
		apiEndpoint: apiEndPt,
		userName:    "syrupSSH",
	}
	aLog.fileName = "logs/sessions/" + prefix + aLog.createTime.Format(logTimeFormat) + ".cast"
	if len(aLog.apikey) > 0 {
		aLog.htClient = &http.Client{
			Timeout: time.Second * 10,
		}
	}
	b, err := json.Marshal(aLog.data)
	if err != nil {
		log.WithField("data", aLog.data).WithError(err).Errorf("Error when marshalling log data")
		return
	}
	b = append(b, '\r', '\n')
	if err = ioutil.WriteFile(aLog.fileName, b, 0600); err != nil {
		log.WithField("path", aLog.fileName).WithError(err).Errorf("Error when writing log file")
		return
	}
	aLog.stdout = make(chan []byte, 100)

	go func(c <-chan []byte) {
		for p := range c {
			now := time.Now()
			diff := now.Sub(aLog.createTime)
			file, err := os.OpenFile(aLog.fileName, os.O_APPEND|os.O_WRONLY, 0666)
			if err != nil {
				log.WithField("path", aLog.fileName).WithError(err).Error("Log write error")
			}
			if escStr, err := json.Marshal(string(p)); err == nil {
				file.Write([]byte(fmt.Sprintf("[%f, \"%v\", %v]\r\n", diff.Seconds(), "o", string(escStr))))
			} else {
				log.WithField("path", aLog.fileName).WithError(err).Error("Log write error")
			}
			file.Close()
		}
		// Upload cast to asciinema.org
		if len(aLog.apikey) > 0 && aLog.elapse > time.Second*5 {
			if url, err := aLog.Upload(); err != nil {
				log.WithError(err).Error("Log failed to upload")
			} else {
				log.WithField("url", url).Info("Log uploaded to URL")
			}

		}
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

// Upload the written file to asciinema server
func (aLog *ASCIICastLog) Upload() (string, error) {
	file, err := os.Open(aLog.fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	filePart, err := writer.CreateFormFile("asciicast", "ascii.cast")
	_, err = io.Copy(filePart, file)
	writer.Close()
	req, _ := http.NewRequest("POST", aLog.apiEndpoint+"/api/asciicasts", buf)
	req.SetBasicAuth(aLog.userName, aLog.apikey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Add("User-Agent", "SyrupSSH/1.0.0")
	rsp, err := aLog.htClient.Do(req)
	body := &bytes.Buffer{}
	_, err = body.ReadFrom(rsp.Body)
	rsp.Body.Close()
	return string(body.Bytes()), err
}

// Close the STDOut keystroke channel for logging
func (aLog *ASCIICastLog) Close() {
	close(aLog.stdout)
	aLog.elapse = time.Since(aLog.createTime)
}

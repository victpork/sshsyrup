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
	stdin       chan []byte
	userName    string
	apikey      string
	apiEndpoint string
	elapse      time.Duration
	htClient    *http.Client
	quit        chan struct{}
}

const (
	logTimeFormat string = "20060102-150405"
)

// NewACastLogger creates a new ASCIICast logger
func NewACastLogger(width, height int, apiEndPt, apiKey string, input io.ReadWriter, params map[string]string) Logger {
	now := time.Now()
	header := asciiCast{
		Version:   2,
		Width:     width,
		Height:    height,
		Timestamp: now.Unix(),
		Title:     fmt.Sprintf("%v@%v - %v", params["USER"], params["SRC"], now.Format(logTimeFormat)),
		Env: map[string]string{
			"TERM":  "vt100",
			"SHELL": "/bin/sh",
		},
	}
	for k, v := range params {
		header.Env[k] = v
	}
	aLog := &ASCIICastLog{
		data:        header,
		readWriter:  input,
		createTime:  now,
		apikey:      apiKey,
		apiEndpoint: apiEndPt,
		userName:    "syrupSSH",
	}
	aLog.fileName = fmt.Sprintf("logs/sessions/%v-%v.cast", params["USER"], aLog.createTime.Format(logTimeFormat))
	if len(aLog.apikey) > 0 {
		aLog.htClient = &http.Client{
			Timeout: time.Second * 10,
		}
	}
	b, err := json.Marshal(aLog.data)
	if err != nil {
		log.WithField("data", aLog.data).WithError(err).Errorf("Error when marshalling log data")
		return nil
	}
	b = append(b, '\r', '\n')
	if err = ioutil.WriteFile(aLog.fileName, b, 0600); err != nil {
		log.WithField("path", aLog.fileName).WithError(err).Errorf("Error when writing log file")
		return nil
	}
	aLog.stdout = make(chan []byte)
	aLog.stdin = make(chan []byte)
	aLog.quit = make(chan struct{})

	go func(in, out <-chan []byte, quit <-chan struct{}) {
	Logloop:
		for {
			select {
			case p := <-in:
				writeLog(aLog.fileName, "i", string(p), aLog.createTime)
			case p := <-out:
				writeLog(aLog.fileName, "o", string(p), aLog.createTime)
			case <-quit:
				break Logloop
			}
		}
		// Upload cast to asciinema.org
		if len(aLog.apikey) > 0 && aLog.elapse > time.Second*5 {
			if url, err := aLog.Upload(); err != nil {
				log.WithError(err).Error("Log failed to upload")
			} else {
				log.WithField("url", url).Info("Log uploaded to URL")
			}

		}
	}(aLog.stdin, aLog.stdout, aLog.quit)
	return aLog
}

func writeLog(fileName, direction, strSeq string, createTime time.Time) {
	now := time.Now()
	diff := now.Sub(createTime)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		log.WithField("path", fileName).WithError(err).Error("Log write error")
		return
	}
	if escStr, err := json.Marshal(strSeq); err == nil {
		file.Write([]byte(fmt.Sprintf("[%f, \"%v\", %v]\r\n", diff.Seconds(), direction, string(escStr))))
	} else {
		log.WithField("path", fileName).WithError(err).Errorf("Cannot parse error string: %v", strSeq)
	}
	file.Close()
}

func (aLog *ASCIICastLog) Read(p []byte) (n int, err error) {
	n, err = aLog.readWriter.Read(p)
	defer func(b []byte) {
		if len(b) > 0 {
			aLog.stdin <- b[:bytes.IndexByte(b, 0)]
		}
	}(p)
	return
}

func (aLog *ASCIICastLog) Write(p []byte) (int, error) {
	defer func(b []byte) {
		if len(b) > 0 {
			aLog.stdout <- b
		}
	}(p)
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
	log.Debug("ASCIICastLog.Close() called")
	close(aLog.stdin)
	close(aLog.stdout)
	close(aLog.quit)
	aLog.elapse = time.Since(aLog.createTime)
}

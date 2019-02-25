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

type asciiCast struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Command   string            `json:"command"`
	Title     string            `json:"title"`
	Env       map[string]string `json:"env"`
}

type AsciinemaHook struct {
	data        asciiCast
	fileName    string
	createTime  time.Time
	userName    string
	apikey      string
	apiEndpoint string
	elapse      time.Duration
	htClient    *http.Client
}

const (
	LogTimeFormat string = "20060102-150405"
	input                = "i"
	output               = "o"
)

// NewAsciinemaHook creates a new Asciinema hook
func NewAsciinemaHook(width, height int, apiEndPt, apiKey string, params map[string]string, fileName string) (LogHook, error) {
	now := time.Now()
	header := asciiCast{
		Version:   2,
		Width:     width,
		Height:    height,
		Timestamp: now.Unix(),
		Title:     fmt.Sprintf("%v@%v - %v", params["USER"], params["SRC"], now.Format(LogTimeFormat)),
		Env: map[string]string{
			"TERM":  "vt100",
			"SHELL": "/bin/sh",
		},
	}
	for k, v := range params {
		header.Env[k] = v
	}
	aLog := &AsciinemaHook{
		data:        header,
		createTime:  now,
		apikey:      apiKey,
		apiEndpoint: apiEndPt,
		userName:    "syrupSSH",
	}
	aLog.fileName = aLog.createTime.Format(fileName)
	if len(aLog.apikey) > 0 {
		aLog.htClient = &http.Client{
			Timeout: time.Second * 10,
		}
	}
	b, err := json.Marshal(aLog.data)
	if err != nil {
		log.WithField("data", aLog.data).WithError(err).Errorf("Error when marshalling log data")
		return nil, err
	}
	b = append(b, '\r', '\n')
	if err = ioutil.WriteFile(aLog.fileName, b, 0600); err != nil {
		log.WithField("path", aLog.fileName).WithError(err).Errorf("Error when writing log file")
		return nil, err
	}

	return aLog, nil
}

func (aLog *AsciinemaHook) Fire(entry *log.Entry) error {
	file, err := os.OpenFile(aLog.fileName, os.O_APPEND|os.O_WRONLY, 0666)
	defer file.Close()
	if err != nil {
		return err
	}
	diff := entry.Time.Sub(aLog.createTime)
	if escStr, err := json.Marshal(entry.Message); err == nil {
		file.WriteString(fmt.Sprintf("[%f, \"%v\", %v]\r\n", diff.Seconds(), entry.Data["dir"], string(escStr)))
	} else {
		return err
	}
	return nil
}

// Upload the written file to asciinema server
func (aLog *AsciinemaHook) upload() (string, error) {
	file, err := os.Open(aLog.fileName)
	defer file.Close()
	if err != nil {
		return "", err
	}
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
	if err != nil {
		return "", err
	}
	body := &bytes.Buffer{}
	_, err = body.ReadFrom(rsp.Body)
	if err != nil {
		return "", err
	}
	rsp.Body.Close()
	return string(body.Bytes()), err
}

// Close the STDOut keystroke channel for logging
func (aLog *AsciinemaHook) Close() error {
	log.Debug("ASCIICastLog.Close() called")
	aLog.elapse = time.Since(aLog.createTime)
	// Upload cast to asciinema.org if key is filled and elapsed time > 5 seconds
	if len(aLog.apikey) > 0 && aLog.elapse > time.Second*5 {
		url, err := aLog.upload()
		if err != nil {
			log.WithError(err).Error("Log failed to upload")
			return err
		}
		log.WithField("url", url).Info("Log uploaded to URL")
	}
	return nil
}

func (aLog *AsciinemaHook) Levels() []log.Level {
	return []log.Level{log.InfoLevel}
}

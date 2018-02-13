package util

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type ElasticHook struct {
	url       string
	formatter log.JSONFormatter
}

type elasticRes struct {
	Result string `json:"result"`
}

func NewElasticHook(endPt, index, pipeline string) log.Hook {
	if strings.LastIndex(endPt, "/") != len(endPt)-1 {
		endPt += "/"
	}

	url := endPt + index + "/_doc/"
	if len(pipeline) > 0 {
		url += "?pipeline=" + pipeline
	}
	return &ElasticHook{
		url:       url,
		formatter: log.JSONFormatter{},
	}
}

func (eh *ElasticHook) Fire(entry *log.Entry) error {
	b, err := eh.formatter.Format(entry)
	if err != nil {
		return nil
	}

	req, _ := http.NewRequest("POST", eh.url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("User-Agent", "SyrupSSH/1.0.0")
	htClient := &http.Client{
		Timeout: time.Second * 10,
	}
	rsp, err := htClient.Do(req)
	body := &bytes.Buffer{}
	_, err = body.ReadFrom(rsp.Body)
	if err != nil {
		return err
	}
	rsp.Body.Close()
	res := elasticRes{}
	json.Unmarshal(body.Bytes(), &res)
	return nil
}

func (eh *ElasticHook) Levels() []log.Level {
	return []log.Level{log.InfoLevel}
}

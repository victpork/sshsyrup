package abuseipdb

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/spf13/viper"
)

const (
	endPoint = "https://www.abuseipdb.com"
)

var (
	reportMap = make(map[string]*Profile)
)

type Profile struct {
	IP      string
	cat     map[int]struct{}
	comment bytes.Buffer
}

func CreateProfile(ip string) {
	reportMap[ip] = createProfile(ip)
}

func AddBehavior(ip, behavior, cmd string) {
	profile := reportMap[ip]
	profile.AddToProfile(behavior, cmd)
}

func UploadReport(ip string) {
	profile := reportMap[ip]
	profile.Report()
}

// ReportIP report to AbuseIPDB regarding IP activities
func reportIP(ip, reason string, cat []int) error {
	apikey := viper.GetString("abuseIPDB.apiKey")
	if len(apikey) == 0 {
		return errors.New("API Key empty")
	}
	arrToStr := func(arr []int) string {
		return strings.Trim(strings.Replace(fmt.Sprint(arr), " ", ",", -1), "[]")
	}
	url := fmt.Sprintf("%v/report/json?key=%v&category=%v&comment=%v&ip=%v", endPoint, apikey, arrToStr(cat), reason, ip)
	fmt.Println(url)
	rsp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}
	if !strings.Contains(string(body), "\"success\":true") {
		return errors.New(string(body))
	}
	return nil
}

func createProfile(ip string) *Profile {
	return &Profile{
		IP:  ip,
		cat: map[int]struct{}{22: struct{}{}},
	}
}

func (p *Profile) AddToProfile(action, cmd string) {
	// Extract URL the string is trying to get
	if strings.Contains(cmd, "wget") || strings.Contains(cmd, "curl") {
		p.cat[20] = struct{}{}
		p.comment.WriteString("Attempt to download malicious scripts; ")
	}
	if action == "shell" {
		p.cat[15] = struct{}{}
		p.comment.WriteString("Attempt to break into ssh; ")
	}
	if action == "mail" {
		p.cat[11] = struct{}{}
		p.comment.WriteString("Attempt to spam via ssh tunnel")
	}
}

func (p *Profile) Report() {
	var catArr []int
	for cat := range p.cat {
		catArr = append(catArr, cat)
	}
	reportIP(p.IP, p.comment.String(), catArr)
}

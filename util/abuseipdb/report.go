package abuseipdb

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	endPoint = "https://www.abuseipdb.com"
)

var (
	reportMap = make(map[string]*Profile)
)

type Category int

const (
	FraudOrder Category = iota + 3
	DDosAttack
	FTPBruteForce
	PingOfDeath
	Phishing
	FraudVoIP
	OpenProxy
	WebSpam
	EmailSpam
	BlogSpam
	VPNIP
	PortScan
	Hacking
	SQLInjection
	Spoofing
	BruteForce
	BadWebBot
	ExploitedHost
	WebAppAttack
	SSH
	IoTTargeted
)

type Profile struct {
	IP      string
	cat     map[Category]struct{}
	comment bytes.Buffer
}

func CreateProfile(ip string) {
	reportMap[ip] = createProfile(ip)
}

func AddCategory(ip string, cat ...Category) {
	reportMap[ip].AddCategory(cat)
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
		cat: map[Category]struct{}{SSH: struct{}{}},
	}
}

func (p *Profile) CheckCommand(cmd string) {
	// Extract URL the string is trying to get
	switch {
	case strings.Contains(cmd, "wget"), strings.Contains(cmd, "curl"):
		p.cat[20] = struct{}{}
		p.comment.WriteString("Attempt to download malicious scripts; ")
	}
}

func (p *Profile) AddCategory(cat []Category) {
	for _, c := range cat {
		p.cat[c] = struct{}{}
	}
}

func (p *Profile) AddReason(reason string) {
	p.comment.WriteString(reason)
}

func (p *Profile) Report() error {
	var catArr []int
	for cat := range p.cat {
		catArr = append(catArr, int(cat))
	}
	return reportIP(p.IP, p.comment.String(), catArr)
}

// LoadRules load report rules file into memory
func LoadRules(ruleFilePath string) error {
	fp, err := os.Open(ruleFilePath)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(fp)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

	}
	return nil
}

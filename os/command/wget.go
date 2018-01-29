package command

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/mkishere/sshsyrup/os"
	"github.com/ogier/pflag"
	"github.com/spf13/afero"
)

type wget struct{}

var (
	flag = pflag.NewFlagSet("arg", pflag.ContinueOnError)
	out  string
)

func init() {
	os.RegisterCommand("wget", wget{})
	flag.StringVar(&out, "O", "", "write documents to FILE.")

}
func (wg wget) GetHelp() string {
	return ""
}

func printTs() string {
	return time.Now().Format("2006-01-02 03:04:05")
}

func (wg wget) Exec(args []string, sys *os.System) int {
	flag.SetOutput(sys.Out())
	err := flag.Parse(args)
	f := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(sys.Out(), "wget: missing URL\nUsage: wget [OPTION]... [URL]...\n\nTry `wget --help' for more options.")
		return 1
	}
	url := strings.TrimSpace(f[0])
	if !strings.Contains(url, "://") {
		url = "http://" + url
	}
	fmt.Fprintf(sys.Out(), "--%v--  %v\n", printTs(), url)
	//urlobj, err := urllib.Parse(url)
	if err != nil {

	}
	resp, err := http.Get(url)
	if err != nil {
		// handle error
	}
	defer resp.Body.Close()
	//resp.Header.Get()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//handle
	}
	if out == "" {
		out = "index.html"
	}
	af := afero.Afero{sys.FSys}
	err = af.WriteFile(out, b, 0666)
	if err != nil {
		//handle
	}
	return 0
}

func (wg wget) Where() string {
	return "/usr/bin/wget"
}

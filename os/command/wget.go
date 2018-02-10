package command

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	urllib "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mkishere/sshsyrup/os"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
)

type wget struct{}

func init() {
	os.RegisterCommand("wget", wget{})

}
func (wg wget) GetHelp() string {
	return ""
}

func printTs() string {
	return time.Now().Format("2006-01-02 03:04:05")
}

func (wg wget) Exec(args []string, sys os.Sys) int {
	flag := pflag.NewFlagSet("arg", pflag.ContinueOnError)
	var out string
	flag.StringVar(&out, "O", "", "write documents to FILE.")

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
	urlobj, err := urllib.Parse(url)
	if err != nil {
		fmt.Fprintln(sys.Out(), "Malformed URL")
		return 1
	}
	if urlobj.Scheme != "http" && urlobj.Scheme != "https" {
		fmt.Fprintf(sys.Out(), "Resolving %v (%v)... failed: Name or service not known.\n", urlobj.Scheme, urlobj.Scheme)
		fmt.Fprintf(sys.Out(), "wget: unable to resolve host address ‘%v’\n", urlobj.Scheme)
		return 1
	}
	fmt.Fprintf(sys.Out(), "--%v--  %v\n", printTs(), url)
	ip, err := net.LookupIP(urlobj.Hostname())
	if err != nil {
		// handle error
	}

	fmt.Fprintf(sys.Out(), "Resolving %v (%v)... %v\n", urlobj.Hostname(), urlobj.Hostname(), ip)
	resp, err := http.Get(url)
	if err != nil {
		// handle error
		return 1
	}
	fmt.Fprintf(sys.Out(), "Connecting to %v (%v)|%v|:80... connected\n", urlobj.Hostname(), urlobj.Hostname(), ip[0])
	mimeType := resp.Header.Get("Content-Type")
	fmt.Fprintln(sys.Out(), "HTTP request sent, awaiting response... 200 OK")
	fmt.Fprintf(sys.Out(), "Length: unspecified [%v]\n", mimeType[:strings.LastIndex(mimeType, ";")])
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//handle
		return 1
	}
	if out == "" {
		out = "index.html"
	}
	szStr := format(len(b))
	fmt.Fprintf(sys.Out(), "Saving to: ‘%v’\n\n", out)
	fmt.Fprintf(sys.Out(), "[ <=>%v ] %v       --.-K/s   in 0.1s\n", strings.Repeat(" ", sys.Width()-38), szStr)
	af := afero.Afero{sys.FSys()}
	err = af.WriteFile(out, b, 0666)
	if err != nil {
		//handle
		return 1
	}
	fmt.Fprintf(sys.Out(), "%v (0.5 KB/s) - ‘%v’ saved[%v]\n", printTs(), out, szStr)
	return 0
}

func (wg wget) Where() string {
	return "/usr/bin/wget"
}

// Idea from https://stackoverflow.com/a/31046325
func format(n int) string {
	in := strconv.Itoa(n)
	out := make([]byte, len(in)+len(in)/3)

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}

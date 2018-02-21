package command

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/mkishere/sshsyrup/os"
)

type uptime struct{}

func init() {
	os.RegisterCommand("uptime", uptime{})
}

func (uptime) GetHelp() string {
	return ""
}

func (uptime) Exec(args []string, sys os.Sys) int {
	currTime := time.Now()
	last5 := rand.Float32() + 9
	last10 := rand.Float32() + 9
	last15 := rand.Float32() + 9
	fmt.Fprintf(sys.Out(), "%v up 3 days,  3 users,  load average: %.2f, %.2f, %.2f\n", currTime.Format("03:04:05"), last5, last10, last15)
	return 0
}

func (uptime) Where() string {
	return "/usr/bin/uptime"
}

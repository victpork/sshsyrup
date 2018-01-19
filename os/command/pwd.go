package command

import (
	"fmt"

	"github.com/mkishere/sshsyrup/os"
)

type pwd struct{}

func init() {
	os.RegisterCommand("pwd", pwd{})
}

func (p pwd) GetHelp() string {
	return ""
}

func (p pwd) Exec(args []string, sys *os.System) int {
	fmt.Fprintln(sys.Out(), sys.Getcwd())
	return 0
}

func (p pwd) Where() string {
	return "/bin/pwd"
}

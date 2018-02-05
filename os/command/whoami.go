package command

import (
	"fmt"

	"github.com/mkishere/sshsyrup/os"
)

type whoami struct{}

func init() {
	os.RegisterCommand("whoami", whoami{})
}

func (whoami) GetHelp() string {
	return ""
}

func (whoami) Exec(args []string, sys os.Sys) int {
	id := sys.CurrentUser()
	u := os.GetUserByID(id)
	fmt.Fprintln(sys.Out(), u.Name)
	return 0
}

func (whoami) Where() string {
	return "/usr/bin/whoami"
}

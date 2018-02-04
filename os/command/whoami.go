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
	// TODO Hardcoded as root till API has way to get UID/GID
	fmt.Fprintln(sys.Out(), "root")
	return 0
}

func (whoami) Where() string {
	return "/usr/bin/whoami"
}

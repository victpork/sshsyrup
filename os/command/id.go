package command

import (
	"fmt"

	"github.com/mkishere/sshsyrup/os"
)

type id struct{}

func init() {
	os.RegisterCommand("id", id{})
}

func (i id) GetHelp() string {
	return ""
}

func (i id) Exec(args []string, sys os.Sys) int {
	uid := sys.CurrentUser()
	gid := sys.CurrentGroup()

	user := os.GetUserByID(uid)
	group := os.GetGroupByID(gid)
	fmt.Fprintf(sys.Out(), "uid=%d(%s) gid=%d(%s) groups=%d(%s)\n", uid, user.Name, gid, group.Name, gid, group.Name)
	return 0
}

func (i id) Where() string {
	return "/bin/uname"
}

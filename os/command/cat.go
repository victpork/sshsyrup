package command

import (
	"fmt"
	"io"
	"os"
	"path"

	honeyos "github.com/mkishere/sshsyrup/os"
)

type cat struct{}

func init() {
	honeyos.RegisterCommand("cat", cat{})
}

func (c cat) GetHelp() string {
	return ""
}

func (c cat) Exec(args []string, sys honeyos.Sys) int {
	if len(args) == 0 {
		return 0
	}
	filePath := args[0]
	if !path.IsAbs(filePath) {
		filePath = path.Join(sys.Getcwd(), filePath)
	}
	f, err := sys.FSys().OpenFile(filePath, os.O_RDONLY, os.ModeType)
	if err != nil {
		if err == os.ErrNotExist {
			fmt.Fprintf(sys.Out(), "cat: %v: No such file or directory\n", args[0])
			return 1
		} else if err == os.ErrPermission {
			fmt.Fprintf(sys.Out(), "cat: %v: Permission denied\n", args[0])
			return 1
		}
	}
	io.Copy(sys.Out(), f)
	return 0
}

func (c cat) Where() string {
	return "/bin/pwd"
}

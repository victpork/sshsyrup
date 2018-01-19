package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mkishere/sshsyrup/os"
)

type ls struct{}

func init() {
	os.RegisterCommand("ls", ls{})
}

func (cmd ls) GetHelp() string {
	return ""
}

func (cmd ls) Where() string {
	return "/bin/ls"
}

func (cmd ls) Exec(args []string, sys *os.System) int {
	var path string
	if len(args) > 0 {
		path = args[0]
	} else {
		path = sys.Getcwd()
	}
	dirList, err := sys.FSys.ReadDir(path)
	if err != nil {
		fmt.Fprintf(sys.Out(), "ls: cannot access %v: No such file or directory\n", path)
		return 1
	}
	// Sort directory list
	dirName := make([]string, 0, len(dirList))
	maxlen := 0
	for k := range dirList {
		if len(k) > maxlen {
			maxlen = len(k)
		}
		dirName = append(dirName, k)
	}
	sort.Strings(dirName)

	itemPerRow := int(sys.Width / (maxlen + 1))

	for i := 0; i < len(dirName); i++ {
		fmt.Fprintf(sys.Out(), "%v%v  ", dirName[i], strings.Repeat(" ", maxlen-len(dirName[i])))
		if (i+1)%itemPerRow == 0 {
			fmt.Fprint(sys.Out(), "\n")
		}
	}
	fmt.Fprint(sys.Out(), "\n")
	return 0
}

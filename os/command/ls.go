package command

import (
	"fmt"
	"os"
	"sort"
	"strings"

	honeyos "github.com/mkishere/sshsyrup/os"
	"github.com/mkishere/sshsyrup/virtualfs"
	"github.com/spf13/pflag"
)

type ls struct{}
type lsFileInfoSort []os.FileInfo

func init() {
	honeyos.RegisterCommand("ls", ls{})
}

func (cmd ls) GetHelp() string {
	return ""
}

func (cmd ls) Where() string {
	return "/bin/ls"
}

func (cmd ls) Exec(args []string, sys honeyos.Sys) int {
	flag := pflag.NewFlagSet("arg", pflag.PanicOnError)
	flag.SetOutput(sys.Out())
	lMode := flag.BoolP("", "l", false, "use a long listing format")
	err := flag.Parse(args)
	f := flag.Args()
	var path string
	if len(f) > 0 {
		path = f[len(f)-1]
	} else {
		path = sys.Getcwd()
	}

	dir, err := sys.FSys().Open(path)
	if err != nil {
		fmt.Fprintf(sys.Out(), "ls: cannot access %v: No such file or directory\n", path)
		return 1
	}
	if *lMode {
		dir, err := dir.Readdir(-1)
		sortDir := lsFileInfoSort(dir)
		if err != nil {
			fmt.Fprintf(sys.Out(), "ls: cannot access %v: No such file or directory\n", path)
			return 1
		}
		sort.Sort(sortDir)
		for _, dir := range sortDir {
			fmt.Fprintln(sys.Out(), getLsString(dir))
		}
	} else {

		// Sort directory list
		dirName, err := dir.Readdirnames(-1)
		if err != nil {
			fmt.Fprintf(sys.Out(), "ls: cannot access %v: No such file or directory\n", path)
			return 1
		}
		maxlen := 0
		for _, d := range dirName {
			if len(d) > maxlen {
				maxlen = len(d)
			}
		}
		sort.Strings(dirName)

		itemPerRow := int(sys.Width()/(maxlen+1) - 1)

		for i := 0; i < len(dirName); i++ {
			if (i+1)%itemPerRow == 0 {
				fmt.Fprint(sys.Out(), "\n")
			}
			fmt.Fprintf(sys.Out(), "%v%v  ", dirName[i], strings.Repeat(" ", maxlen-len(dirName[i])))
		}
		fmt.Fprint(sys.Out(), "\n")
	}
	return 0
}

func (fi lsFileInfoSort) Len() int { return len(fi) }

func (fi lsFileInfoSort) Swap(i, j int) { fi[i], fi[j] = fi[j], fi[i] }

func (fi lsFileInfoSort) Less(i, j int) bool { return fi[i].Name() < fi[j].Name() }

func getLsString(fi os.FileInfo) string {
	uid, gid, _, _ := virtualfs.GetExtraInfo(fi)
	uName := honeyos.GetUserByID(uid).Name
	gName := honeyos.GetGroupByID(gid).Name

	size := fi.Size()
	if fi.IsDir() {
		size = 4096
	}
	return fmt.Sprintf("%v    1 %-8s %-8s %8d %v %v", strings.ToLower(fi.Mode().String()), uName, gName,
		size, fi.ModTime().Format("Jan 02 15:04"), fi.Name())
}

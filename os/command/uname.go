package command

import (
	"fmt"

	"github.com/mkishere/sshsyrup/os"
	"github.com/spf13/pflag"
)

type uname struct{}

func init() {
	os.RegisterCommand("uname", uname{})
}

func (un uname) GetHelp() string {
	return ""
}

func (un uname) Exec(args []string, sys os.Sys) int {
	flag := pflag.NewFlagSet("arg", pflag.PanicOnError)
	a := flag.BoolP("all", "a", false, "print all information, in the following order, except omit -p and -i if unknown:")
	_ = flag.BoolP("kernel-name", "s", false, "print the kernel name")
	_ = flag.BoolP("kernel-release", "r", false, "print the kernel release")
	flag.Parse(args)

	if *a {
		fmt.Fprintln(sys.Out(), "Linux spr1739 4.4.0-43-generic #129-Ubuntu SMP Thu Mar 17 20:17:14 UTC 2017 x86_64 x86_64 x86_64 GNU/Linux")
	} else {
		fmt.Fprintln(sys.Out(), "Linux")
	}
	return 0
}

func (un uname) Where() string {
	return "/bin/uname"
}

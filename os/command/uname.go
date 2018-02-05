package command

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mkishere/sshsyrup/os"
	"github.com/spf13/pflag"
)

type uname struct{}

const (
	unameKName  = "Linux"
	unameKRel   = "4.4.0-43-generic"
	unameKVer   = "#129-Ubuntu SMP Thu Mar 17 20:17:14 UTC 2017"
	unameMach   = "x86-64"
	unameProc   = "x86-64"
	unameHWPlat = "x86-64"
	unameOS     = "GNU/Linux"
)

func init() {
	os.RegisterCommand("uname", uname{})
}

func (un uname) GetHelp() string {
	return ""
}

func (un uname) Exec(args []string, sys os.Sys) int {
	flag := pflag.NewFlagSet("arg", pflag.ContinueOnError)
	all := flag.BoolP("all", "a", false,
		"print all information, in the following order,\n                              except omit -p and -i if unknown:")
	kName := flag.BoolP("kernel-name", "s", false, "print the kernel name")
	nName := flag.BoolP("nodename", "n", false, "print the network node hostname")
	kRel := flag.BoolP("kernel-release", "r", false, "print the kernel release")
	kVer := flag.BoolP("kernel-version", "v", false, "print the kernel version")
	mach := flag.BoolP("machine", "m", false, "print the machine hardware name")
	proc := flag.BoolP("processor", "p", false, "print the processor type or \"unknown\"")
	hwPlat := flag.BoolP("hardware-platform", "i", false, "print the hardware platform or \"unknown\"")
	help := flag.Bool("help", false, "display this help and exit")
	ver := flag.Bool("version", false, "output version information and exit")
	os := flag.BoolP("operating-system", "o", false, "print the operating system")
	flag.SetOutput(sys.Out())
	flag.Usage = func() {
		fmt.Fprintf(sys.Out(), "Usage: uname [OPTION]...\n")
		fmt.Fprintf(sys.Out(), "Print certain system information.  With no OPTION, same as -s.\n\n")
		flag.PrintDefaults()
		fmt.Fprintln(sys.Out(), "\nReport uname bugs to bug-coreutils@gnu.org")
		fmt.Fprintln(sys.Out(), "GNU coreutils home page: <http://www.gnu.org/software/coreutils/>")
		fmt.Fprintln(sys.Out(), "General help using GNU software: <http://www.gnu.org/gethelp/>")
		fmt.Fprintln(sys.Out(), "For complete documentation, run: info coreutils 'uname invocation'")
	}
	err := flag.Parse(args)
	if err != nil {
		return 1
	}
	if *all {
		fmt.Fprintf(sys.Out(), "%v %v %v %v %v %v %v %v\n", unameKName, sys.Hostname(), unameKRel,
			unameKVer, unameMach, unameProc, unameHWPlat, unameOS)
	} else if len(args) == 0 {
		fmt.Fprintln(sys.Out(), unameKName)
	} else if *ver {
		fmt.Fprint(sys.Out(), un.PrintVer())
	} else if *help {
		flag.Usage()
	} else {
		var uNameStr bytes.Buffer
		switch {
		case *kName:
			uNameStr.WriteString(unameKName)
			fallthrough
		case *nName:
			uNameStr.WriteString(" " + sys.Hostname())
			fallthrough
		case *kRel:
			uNameStr.WriteString(" " + unameKRel)
			fallthrough
		case *kVer:
			uNameStr.WriteString(" " + unameKVer)
			fallthrough
		case *mach:
			uNameStr.WriteString(" " + unameMach)
			fallthrough
		case *proc:
			uNameStr.WriteString(" " + unameProc)
			fallthrough
		case *hwPlat:
			uNameStr.WriteString(" " + unameHWPlat)
			fallthrough
		case *os:
			uNameStr.WriteString(" " + unameOS)
		}
		fmt.Fprintln(sys.Out(), strings.TrimSpace(uNameStr.String()))
	}
	return 0
}

func (un uname) Where() string {
	return "/bin/uname"
}

func (un uname) PrintVer() string {
	return "uname (GNU coreutils) 8.21\n" +
		"Copyright (C) 2013 Free Software Foundation, Inc.\n" +
		"License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.\n" +
		"This is free software: you are free to change and redistribute it.\n" +
		"There is NO WARRANTY, to the extent permitted by law.\n\n" +
		"Written by David MacKenzie."
}

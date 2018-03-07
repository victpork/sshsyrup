package os

import (
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/mkishere/sshsyrup/util/termlogger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type Shell struct {
	log        *log.Entry
	termSignal chan<- int
	terminal   *terminal.Terminal
	sys        *System
	DelayFunc  func()
}

func NewShell(sys *System, ipSrc string, log *log.Entry, termSignal chan<- int) *Shell {

	return &Shell{
		log:        log,
		termSignal: termSignal,
		sys:        sys,
	}
}

func (sh *Shell) HandleRequest(hook termlogger.LogHook) {

	tLog := termlogger.NewLogger(hook, sh.sys.In(), sh.sys.Out(), sh.sys.Err())
	defer tLog.Close()

	sh.terminal = terminal.NewTerminal(struct {
		io.Reader
		io.Writer
	}{
		tLog.In(),
		tLog.Out(),
	}, "$ ")
	defer func() {
		if r := recover(); r != nil {
			sh.log.Errorf("Recovered from panic %v", r)
			sh.termSignal <- 1
		}
	}()
	shellParser := shellwords.NewParser()
	shellParser.ParseBacktick = false
	for {
		cmd, err := sh.terminal.ReadLine()
		if len(strings.TrimSpace(cmd)) > 0 {
			sh.log.WithField("cmd", cmd).Infof("User input command %v", cmd)
		}
		if sh.DelayFunc != nil {
			sh.DelayFunc()
		}
		if err != nil {
			if err.Error() == "EOF" {
				sh.log.WithError(err).Info("Client disconnected from server")
				sh.termSignal <- 0
				return
			}
			sh.log.WithError(err).Error("Error when reading terminal")
			break
		}

		pos := 0
		for pos < len(cmd) {
			cmdList, err := shellParser.Parse(cmd[pos:])
			if err != nil {
				sh.terminal.Write([]byte(cmd[pos:] + ": command not found"))
				break
			}
			for i, cmdComp := range cmdList {
				if strings.Contains(cmdComp, "=") {
					envVar := strings.SplitN(cmdComp, "=", 1)
					sh.sys.SetEnv(envVar[0], envVar[1])
				} else {
					sh.ExecCmd(strings.Join(cmdList[i:], " "), tLog)
					break
				}
			}
			if shellParser.Position == -1 {
				break
			}
			pos += shellParser.Position + 1
		}
	}
}

func (sh *Shell) SetSize(width, height int) error {
	sh.sys.width = width
	sh.sys.height = height
	return sh.terminal.SetSize(width, height)
}

func (sh *Shell) ExecCmd(cmd string, tLog termlogger.StdIOErr) {
	cmd = strings.TrimSpace(cmd)
	switch {

	case strings.TrimSpace(cmd) == "":
		//Do nothing
	case cmd == "logout", cmd == "exit":
		sh.log.Infof("User logged out")
		sh.termSignal <- 0
		return
	case strings.HasPrefix(cmd, "cd"):
		args := strings.Split(cmd, " ")
		if len(args) > 1 {
			err := sh.sys.Chdir(args[1])
			if err != nil {
				sh.terminal.Write([]byte(fmt.Sprintf("-bash: cd: %v: No such file or directory\n", args[1])))
			}
		}
	case strings.HasPrefix(cmd, "export"):

	default:
		args := strings.Split(cmd, " ")
		n, err := sh.sys.exec(args[0], args[1:], tLog)
		if err != nil {
			sh.terminal.Write([]byte(fmt.Sprintf("%v: command not found\n", args[0])))
		} else {
			sh.sys.envVars["?"] = string(n)
		}
	}
}

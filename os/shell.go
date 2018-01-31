package os

import (
	"fmt"
	"strings"

	"github.com/mkishere/sshsyrup/util/termlogger"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type Shell struct {
	log        *log.Entry
	termSignal chan<- int
	terminal   *terminal.Terminal
	sys        *System
}

func NewShell(sys *System, ipSrc string, log *log.Entry, termSignal chan<- int) *Shell {

	return &Shell{
		log:        log,
		termSignal: termSignal,
		sys:        sys,
	}
}

func (sh *Shell) HandleRequest(hook termlogger.LogHook) {
	tLog := termlogger.NewLogger(hook, sh.sys.IOStream())
	defer tLog.Close()

	sh.terminal = terminal.NewTerminal(tLog, "$ ")
	defer func() {
		if r := recover(); r != nil {
			sh.log.Errorf("Recovered from panic %v", r)
			sh.termSignal <- 1
		}
	}()
cmdLoop:
	for {
		cmd, err := sh.terminal.ReadLine()
		sh.log.WithField("cmd", cmd).Infof("User input command %v", cmd)
		cmd = strings.TrimSpace(cmd)
		switch {
		case err != nil:
			if err.Error() == "EOF" {
				sh.log.Info("EOF received from client")
				sh.termSignal <- 0
				return
			} else {
				sh.log.WithError(err).Error("Error when reading terminal")
			}
			break cmdLoop
		case strings.TrimSpace(cmd) == "":
			//Do nothing
		case cmd == "logout", cmd == "quit":
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
			// TODO: parse script

			args := strings.SplitN(cmd, " ", 2)
			n, err := sh.sys.exec(args[0], args[1:], tLog)
			if err != nil {
				sh.terminal.Write([]byte(fmt.Sprintf("%v: command not found\n", args[0])))
			} else {
				sh.sys.envVars["?"] = string(n)
			}
		}
	}
}

func (sh *Shell) SetSize(width, height int) error {
	sh.sys.width = width
	sh.sys.height = height
	return sh.terminal.SetSize(width, height)
}

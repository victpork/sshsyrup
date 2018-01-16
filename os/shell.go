package shell

import (
	"fmt"
	"io"
	"os"
	pathlib "path"
	"strings"

	"github.com/mkishere/sshsyrup/util/termlogger"

	"github.com/mkishere/sshsyrup/virtualfs"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

type Shell struct {
	cwd        string
	fs         *virtualfs.VirtualFS
	funcMap    map[string]Command
	envVars    map[string]string
	iostream   io.ReadWriter
	log        *log.Entry
	sessionLog *termlogger.Logger
	width      int
	height     int
}

type Command interface {
	Exec([]string) int
}

func NewShell(iostream io.ReadWriter, fsys *virtualfs.VirtualFS, width, height int, user, ipSrc string, log *log.Entry) *Shell {
	fMap := make(map[string]Command)
	return &Shell{
		iostream: iostream,
		cwd:      "/home/" + user,
		fs:       fsys,
		funcMap:  fMap,
		width:    width,
		height:   height,
		log:      log,
	}
}

func (sh *Shell) HandleRequest(tLog termlogger.Logger) {
	t := terminal.NewTerminal(sh.iostream, "> ")
	/* tLog := termlogger.NewACastLogger(sh.width, sh.height,
	config.AcinemaAPIEndPt, config.AcinemaAPIKey, sh.iostream, asciiLogParams) */
	defer tLog.Close()
cmdLoop:
	for {
		cmd, err := t.ReadLine()
		sh.log.WithField("cmd", cmd).Infof("User input command %v", cmd)
		switch {
		case err != nil:
			if err.Error() == "EOF" {
				sh.log.Info("EOF received from client")
			} else {
				sh.log.WithError(err).Error("Error when reading terminal")
			}
			break cmdLoop
		case strings.TrimSpace(cmd) == "":
			//Do nothing
		case cmd == "logout", cmd == "quit":
			sh.log.Infof("User logged out")
			return
		default:
			args := strings.SplitN(cmd, " ", 2)
			//sh.Exec(args[0], args[1:])
			t.Write([]byte(fmt.Sprintf("%v: command not found\n", args[0])))
		}
	}
}

func (sh *Shell) input(line string) error {
	switch {
	case strings.HasPrefix(line, "cd "):
		err := sh.chdir(line[3:])
		if err != nil {
			return err
		}
	}
	return nil
}

func (sh *Shell) getcwd() string {
	return sh.cwd
}

func (sh *Shell) chdir(path string) error {
	if !pathlib.IsAbs(path) {
		path = sh.cwd + "/" + path
	}
	if !sh.fs.IsExist(path) {
		return os.ErrNotExist
	}
	sh.cwd = path
	return nil
}

func (sh *Shell) Exec(path string, args []string) (io.ReadWriter, error) {
	cmd := pathlib.Base(path)
	if execFunc, ok := sh.funcMap[cmd]; ok {
		execFunc.Exec(args)
	} else {

	}
	return nil, nil
}

func (sh *Shell) SetSize(width, height int) {

}

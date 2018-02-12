package os

import (
	"bytes"
	"fmt"
	"io"
	"os"
	pathlib "path"

	"github.com/mkishere/sshsyrup/util/termlogger"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

var (
	crlf = []byte{'\r', '\n'}
)

var (
	funcMap      = make(map[string]Command)
	fakeFuncList = make(map[string]struct{})
)

var (
	errMsgList = map[string]struct{}{
		"Segmentation fault": struct{}{},
		"Permission denied":  struct{}{},
	}
)

// Command interface allow classes to simulate real executable
// that have access to standard I/O, filesystem, arguments, EnvVars,
// and cwd
type Command interface {
	GetHelp() string
	Exec(args []string, sys Sys) int
	Where() string
}

// System provides what most of os/sys does in the honeyport
type System struct {
	userId        int
	cwd           string
	fSys          afero.Fs
	sshChan       ssh.Channel
	envVars       map[string]string
	width, height int
	log           *log.Entry
	sessionLog    termlogger.LogHook
	hostName      string
}

type Sys interface {
	Getcwd() string
	Chdir(path string) error
	In() io.Reader
	Out() io.Writer
	Err() io.Writer
	Environ() (env []string)
	SetEnv(key, value string) error
	FSys() afero.Fs
	Width() int
	Height() int
	CurrentUser() int
	CurrentGroup() int
	Hostname() string
}
type stdoutWrapper struct {
	io.Writer
}

type sysLogWrapper struct {
	termlogger.StdIOErr
	*System
}

func (sys *sysLogWrapper) In() io.Reader  { return sys.StdIOErr.In() }
func (sys *sysLogWrapper) Out() io.Writer { return stdoutWrapper{sys.StdIOErr.Out()} }
func (sys *sysLogWrapper) Err() io.Writer { return stdoutWrapper{sys.StdIOErr.Err()} }

// NewSystem initializer a system object containing current user context: ID,
// home directory, terminal dimensions, etc.
func NewSystem(user, host string, fs afero.Fs, channel ssh.Channel, width, height int, log *log.Entry) *System {
	if _, exists := IsUserExist(user); !exists {
		CreateUser(user, "password")
	}
	aferoFs := afero.Afero{fs}
	if exists, _ := aferoFs.DirExists(usernameMapping[user].Homedir); !exists {
		aferoFs.MkdirAll(usernameMapping[user].Homedir, 0755)
	}

	return &System{
		cwd:      usernameMapping[user].Homedir,
		fSys:     aferoFs,
		envVars:  map[string]string{},
		sshChan:  channel,
		width:    width,
		height:   height,
		log:      log,
		userId:   usernameMapping[user].UID,
		hostName: host,
	}
}

// Getcwd gets current working directory
func (sys *System) Getcwd() string {
	return sys.cwd
}

// Chdir change current working directory
func (sys *System) Chdir(path string) error {
	if !pathlib.IsAbs(path) {
		path = sys.cwd + "/" + path
	}
	path = pathlib.Clean(path)
	if exists, err := afero.DirExists(sys.fSys, path); err == nil && !exists {
		return os.ErrNotExist
	} else if err != nil {
		return err
	}
	sys.cwd = path
	return nil
}

func (sys *System) CurrentUser() int { return sys.userId }

func (sys *System) CurrentGroup() int {
	u := GetUserByID(sys.userId)
	return u.GID
}

func (sys *System) Hostname() string {
	return sys.hostName
}

// In returns a io.Reader that represent stdin
func (sys *System) In() io.Reader { return sys.sshChan }

// Out returns a io.Writer that represent stdout
func (sys *System) Out() io.Writer {
	return stdoutWrapper{sys.sshChan}
}

func (sys *System) Err() io.Writer {
	return stdoutWrapper{sys.sshChan.Stderr()}
}

func (sys *System) IOStream() io.ReadWriter { return sys.sshChan }

func (sys *System) FSys() afero.Fs { return sys.fSys }

func (sys *System) Width() int { return sys.width }

func (sys *System) Height() int { return sys.height }

// Write replace \n with \r\n before writing to the underlying io.Writer.
// Copied from golang.org/x/crypto/ssh/terminal
func (sw stdoutWrapper) Write(buf []byte) (n int, err error) {
	for len(buf) > 0 {
		i := bytes.IndexByte(buf, '\n')
		todo := len(buf)
		if i >= 0 {
			todo = i
		}

		var nn int
		nn, err = sw.Writer.Write(buf[:todo])
		n += nn
		if err != nil {
			return n, err
		}
		buf = buf[todo:]

		if i >= 0 {
			if _, err = sw.Writer.Write(crlf); err != nil {
				return n, err
			}
			n++
			buf = buf[1:]
		}
	}

	return n, nil
}

func (sys *System) Environ() (env []string) {
	env = make([]string, 0, len(sys.envVars))
	for k, v := range sys.envVars {
		env = append(env, fmt.Sprintf("%v=%v", k, v))
	}
	return
}

func (sys *System) SetEnv(key, value string) error {
	sys.envVars[key] = value
	return nil
}

func (sys *System) Exec(path string, args []string) (int, error) {
	return sys.exec(path, args, nil)
}

func (sys *System) exec(path string, args []string, io termlogger.StdIOErr) (int, error) {
	cmd := pathlib.Base(path)
	if execFunc, ok := funcMap[cmd]; ok {

		defer func() {
			if r := recover(); r != nil {
				sys.log.WithFields(log.Fields{
					"cmd":   path,
					"args":  args,
					"error": r,
				}).Error("Command has crashed")
				sys.Err().Write([]byte("Segmentation fault\n"))
			}
		}()
		var res int
		// If logger is not nil, redirect IO to it
		if io != nil {
			loggedSys := &sysLogWrapper{io, sys}
			res = execFunc.Exec(args, loggedSys)
		} else {
			res = execFunc.Exec(args, sys)
		}
		return res, nil
	} else if _, inList := fakeFuncList[cmd]; inList {
		// Print random error message
		// Make use of golang map rnadom nature :)
		for msg := range errMsgList {
			sys.Err().Write([]byte(msg + "\n"))
			break
		}
		return 1, nil
	}

	return 127, &os.PathError{Op: "exec", Path: path, Err: os.ErrNotExist}
}

// RegisterCommand puts the command implementation into map so
// it can be invoked from command line
func RegisterCommand(name string, cmd Command) {
	funcMap[name] = cmd
	funcMap[cmd.Where()] = cmd
}

// RegisterFakeCommand put commands into register so that when
// typed in terminal they will print out SegFault
func RegisterFakeCommand(cmdList []string) {
	for i := range cmdList {
		fakeFuncList[cmdList[i]] = struct{}{}
	}
}

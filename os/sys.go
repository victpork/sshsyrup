package os

import (
	"bytes"
	"fmt"
	"io"
	"os"
	pathlib "path"

	"github.com/spf13/afero"
)

var (
	crlf = []byte{'\r', '\n'}
)

// Command interface allow classes to simulate real executable
// that have access to standard I/O, filesystem, arguments, EnvVars,
// and cwd
type Command interface {
	GetHelp() string
	Exec(args []string, sys *System) int
	Where() string
}

// System emulates what most of os/sys does in the honeyport
type System struct {
	cwd           string
	FSys          afero.Fs
	io            io.ReadWriter
	envVars       map[string]string
	Width, Height int
}

type stdoutWrapper struct {
	out io.Writer
}

func NewSystem(user string, fs afero.Fs, io io.ReadWriter) *System {

	return &System{
		cwd:  "/home/" + user,
		FSys: fs,
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
	if exists, err := afero.DirExists(sys.FSys, path); err == nil && !exists {
		return os.ErrNotExist
	} else if err != nil {
		return err
	}
	sys.cwd = path
	return nil
}

// In returns a io.Reader that represent stdin
func (sys *System) In() io.Reader { return sys.io }

// Out returns a io.Writer that represent stdout
func (sys *System) Out() io.Writer {
	return stdoutWrapper{out: sys.io}
}

func (sys *System) IOStream() io.ReadWriter { return sys.io }

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
		nn, err = sw.out.Write(buf[:todo])
		n += nn
		if err != nil {
			return n, err
		}
		buf = buf[todo:]

		if i >= 0 {
			if _, err = sw.out.Write(crlf); err != nil {
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

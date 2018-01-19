package os

import (
	"bytes"
	"io"
	"os"
	pathlib "path"

	"github.com/mkishere/sshsyrup/virtualfs"
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
	FSys          *virtualfs.VirtualFS
	io            io.ReadWriter
	envVars       map[string]string
	Width, Height int
}

type stdoutWrapper struct {
	out io.Writer
}

func NewSystem(user string, fs *virtualfs.VirtualFS, io io.ReadWriter) *System {

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
	if !sys.FSys.IsExist(path) {
		return os.ErrNotExist
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

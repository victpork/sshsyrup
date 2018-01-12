package main

import (
	"io"
	"os"
	pathlib "path"
	"strings"

	"github.com/mkishere/sshsyrup/virtualfs"
)

type Shell struct {
	cwd      string
	fs       *virtualfs.VirtualFS
	funcMap  map[string]Command
	envVars  map[string]string
	iostream io.ReadWriter
}

type Command interface {
	Exec([]string) int
}

func NewShell(homedir string, iostream io.ReadWriter, fsys *virtualfs.VirtualFS) *Shell {
	fMap := make(map[string]Command)
	return &Shell{
		cwd:     homedir,
		fs:      fsys,
		funcMap: fMap,
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

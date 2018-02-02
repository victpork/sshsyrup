package os

import (
	"bufio"
	"io"
	"os"
	pathlib "path"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
)

type SCP struct {
	ReadWriter io.ReadWriter
	Fs         afero.Fs
}

const (
	scp_OK byte = iota
	scp_ERR
	scp_FATAL
)

func (scp *SCP) Main(args []string, quit chan<- int) {
	flag := pflag.NewFlagSet("args", pflag.ContinueOnError)
	flag.SetOutput(scp.ReadWriter)
	toMode := flag.BoolP("to", "t", false, "To(Sink) mode")
	fromMode := flag.BoolP("from", "f", false, "From(Source) mode")
	recursive := flag.BoolP("", "r", false, "Recursive")
	flag.MarkHidden("to")
	flag.MarkHidden("from")
	err := flag.Parse(args)
	if err != nil {
		quit <- 1
		return
	}
	if *toMode && *fromMode || !*toMode && !*fromMode {
		quit <- 1
		return
	} else if *toMode {
		scp.sinkMode(flag.Arg(0), *recursive, quit)
	} else if *fromMode {
		scp.sourceMode(flag.Arg(0), *recursive, quit)
	}
}

// sinkMode is the function to receive files/commands from the client side
func (scp *SCP) sinkMode(path string, isRecursive bool, quit chan<- int) {
	buf := bufio.NewReader(scp.ReadWriter)
	scp.sendReply(scp_OK)
	cwd := path
	for {
		cmd, err := buf.ReadString('\n')
		if err != nil && err != io.EOF {
			log.WithError(err).Error("Error")
			return
		}
		if err == io.EOF {
			quit <- 0
			break
		}
		log.Info(cmd, []byte(cmd))
		switch cmd[0] {
		case 'C':
			args := strings.Split(cmd[:len(cmd)-1], " ")
			if len(args) < 3 {
				scp.sendReply(scp_ERR)
				continue
			}
			mode, err := strconv.ParseInt(args[0][1:], 8, 0)
			size, err := strconv.ParseInt(args[1], 10, 0)
			if err != nil {
				scp.sendReply(scp_ERR)
				continue
			}

			realPath := pathlib.Join(cwd, args[2])
			f, err := scp.Fs.OpenFile(realPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(mode))
			if err != nil {
				scp.sendReply(scp_ERR)
				continue
			}

			scp.sendReply(scp_OK)
			n, err := io.CopyN(f, buf, int64(size))
			log.Infof("Received file %v bytes", n)
			if err != nil && err != io.EOF || n != size {
				scp.sendReply(scp_ERR)
				continue
			}
			// Discard the EOF following the file
			_, err = buf.Discard(1)
			if err != nil {
				scp.sendReply(scp_ERR)
			}
			scp.sendReply(scp_OK)
			f.Close()
		case 'D':
			args := strings.Split(cmd[:len(cmd)-1], " ")
			if len(args) < 3 {
				scp.sendReply(scp_ERR)
				continue
			}
			mode, err := strconv.ParseInt(args[0][1:], 8, 0)
			if err != nil {
				scp.sendReply(scp_ERR)
				continue
			}
			cwd = pathlib.Join(path, args[2])
			err = scp.Fs.MkdirAll(cwd, os.FileMode(mode))
			if err != nil {
				scp.sendReply(scp_ERR)
				continue
			}
			scp.sendReply(scp_OK)
		case 'E':
			cwd = path
			scp.sendReply(scp_OK)
		}
	}

}

// sourceMode is the function to send files/commands to the client side
func (scp *SCP) sourceMode(path string, isRecursive bool, quit chan<- int) {

}

func (scp *SCP) sendReply(reply byte) {
	log.Debugf("Sending %v", reply)
	scp.ReadWriter.Write([]byte{reply})
}

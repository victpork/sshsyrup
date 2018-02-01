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
)

type SCP struct {
	ReadWriter io.ReadWriter
	Fs         afero.Fs
}

const (
	scp_OK    byte = 0x00
	scp_ERR        = 0x01
	scp_FATAL      = 0x02
)

func (scp *SCP) sendReply(reply byte) {
	log.Infof("Sending %v", reply)
	scp.ReadWriter.Write([]byte{reply})
}

func (scp *SCP) SinkMode(path string, quit chan<- int) {
	buf := bufio.NewReader(scp.ReadWriter)
	scp.sendReply(scp_OK)
	for {
		cmd, err := buf.ReadString('\n')
		log.Info(cmd)
		if err != nil && err != io.EOF {
			log.WithError(err).Error("Error")
			return
		}
		if err == io.EOF {
			quit <- 0
			break
		}
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

			realPath := pathlib.Join(path, args[2])
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
			//f.Write(b)
			scp.sendReply(scp_OK)
			f.Close()
			c, err := buf.ReadByte()
			if c == 'E' && err != nil {
				scp.sendReply(scp_OK)
				quit <- 0
			}
		case 'D':
		case 'E':

		}
	}
}

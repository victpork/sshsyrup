package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	pathlib "path"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type SCP struct {
	Fs  afero.Fs
	log *log.Entry
	buf *bufio.ReadWriter
}

const (
	scp_OK byte = iota
	scp_ERR
	scp_FATAL
)

// NewSCP creates SCP instance for doing scp operations
func NewSCP(ch io.ReadWriter, fs afero.Fs, log *log.Entry) *SCP {
	scp := &SCP{
		Fs:  fs,
		log: log,
	}
	bufReader := bufio.NewReader(ch)
	bufWriter := bufio.NewWriter(ch)
	scp.buf = bufio.NewReadWriter(bufReader, bufWriter)
	return scp
}

// Main is the function for outside (the main routine) to invoke scp
func (scp *SCP) Main(args []string, quit chan<- int) {
	flag := pflag.NewFlagSet("args", pflag.ContinueOnError)
	flag.SetOutput(scp.buf)
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

	var res int
	if *toMode && *fromMode || !*toMode && !*fromMode {
		quit <- 1
		return
	} else if *toMode {
		res = scp.sinkMode(flag.Arg(0), *recursive)
	} else if *fromMode {
		res = scp.sourceMode(flag.Arg(0), *recursive)
	}
	quit <- res
}

// sinkMode is the function to receive files/commands from the client side
func (scp *SCP) sinkMode(path string, isRecursive bool) int {
	scp.sendReply(scp_OK)
	cwd := path
	for {
		cmd, err := scp.buf.ReadString('\n')
		if err != nil && err != io.EOF {
			scp.log.WithError(err).Error("Error")
			return 1
		}
		if err == io.EOF {
			return 0
			break
		}
		scp.log.Debug(cmd, []byte(cmd))
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
			// Reject file size larger than limit
			if limit := int64(viper.GetSizeInBytes("server.receiveFileSizeLimit")); limit > 0 && size > limit {
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
			n, err := io.CopyN(f, scp.buf, int64(size))
			scp.log.WithFields(log.Fields{
				"path": realPath,
				"size": n,
			}).Infof("Server Received file %v %v bytes", realPath, n)
			if err != nil && err != io.EOF || n != size {
				scp.sendReply(scp_ERR)
				continue
			}
			// Discard the EOF following the file
			_, err = scp.buf.Discard(1)
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
			cwd = pathlib.Join(cwd, args[2])
			err = scp.Fs.MkdirAll(cwd, 0755)
			if err != nil {
				scp.sendReply(scp_ERR)
				continue
			}
			scp.log.WithField("path", cwd).Infof("Server Created directory with mode %v", mode)
			scp.sendReply(scp_OK)
		case 'E':
			cwd = pathlib.Dir(cwd)
			scp.sendReply(scp_OK)
		case 'T':
			scp.sendReply(scp_OK)
		default:
			scp.sendReply(scp_ERR)
		}

	}
	return 0
}

// sourceMode is the function to send files/commands to the client side
func (scp *SCP) sourceMode(path string, isRecursive bool) int {
	cwd := path
	dirLevel := 0
	if isRecursive {
		fs := afero.Afero{scp.Fs}
		fs.Walk(path, func(p string, info os.FileInfo, err error) error {
			p = strings.Replace(p, "\\", "/", -1)
			if !strings.HasPrefix(p, cwd) {
				scp.buf.WriteString("E\n")
				scp.log.Debug("Server sending cmd:E")
				if b, err := scp.buf.ReadByte(); b != 0 || err != nil && err != io.EOF {
					if b != 0 {
						err = errors.New("Client side error")
					}
					return err
				}
				dirLevel--
			}
			if info.IsDir() {
				scp.buf.WriteString(fmt.Sprintf("D%04o 0 %v\n", info.Mode()&os.ModePerm, info.Name()))
				scp.log.Debugf("Server sending cmd:D%04o 0 %v", info.Mode()&os.ModePerm, info.Name())
				scp.buf.Flush()
				if b, err := scp.buf.ReadByte(); b != 0 || err != nil && err != io.EOF {
					if b != 0 {
						err = errors.New("Client side error")
					}
					return err
				}
				cwd = p
				dirLevel++
			} else {
				err := scp.sendFile(p, info)
				if err != nil {
					return err
				}
				cwd = pathlib.Dir(p)
			}
			return nil
		})
		for dirLevel > 0 {
			scp.buf.WriteString("E\n")
			scp.buf.Flush()
			scp.log.Debug("Server sending cmd:E")
			if b, err := scp.buf.ReadByte(); b != 0 || err != nil && err != io.EOF {
				if b != 0 {
					return 1
				}
			}
			dirLevel--
		}
	} else {
		fi, err := scp.Fs.Stat(path)
		if err != nil {
			scp.sendReply(scp_ERR)
		}
		err = scp.sendFile(path, fi)
		if err != nil {
			scp.sendReply(scp_ERR)
		}
	}
	return 0
}

func (scp *SCP) sendReply(reply byte) {
	scp.log.Debugf("Server Replying %v", reply)
	scp.buf.Write([]byte{reply})
	scp.buf.Flush()
}

func (scp *SCP) sendFile(p string, fi os.FileInfo) error {
	scp.buf.WriteString(fmt.Sprintf("C%04o %v %v\n", fi.Mode(), fi.Size(), fi.Name()))
	scp.log.Debugf("Server sending cmd:C%04o %v %v", fi.Mode(), fi.Size(), fi.Name())
	scp.log.WithField("file", p).Info("Server sending file")
	scp.buf.Flush()
	if b, err := scp.buf.ReadByte(); b != 0 || err != nil && err != io.EOF {
		if b != 0 {
			err = errors.New("Client side error")
		}
		return err
	}
	f, err := scp.Fs.OpenFile(p, os.O_RDONLY, 0777)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(scp.buf, f)
	if err != nil {
		return err
	}
	err = scp.buf.WriteByte(0)
	scp.buf.Flush()
	if err != nil {
		return err
	}
	if b, err := scp.buf.ReadByte(); err != nil || b != scp_OK {
		return err
	}
	return nil
}

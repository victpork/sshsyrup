package sftp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	pathlib "path"

	honeyos "github.com/mkishere/sshsyrup/os"
	"github.com/mkishere/sshsyrup/virtualfs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type sftpMsg struct {
	Length  uint32
	Type    ReqType
	ReqID   uint32
	Payload []byte
}

type ReqType byte

const (
	SSH_FXP_INIT = iota + 1
	SSH_FXP_VERSION
	SSH_FXP_OPEN
	SSH_FXP_CLOSE
	SSH_FXP_READ
	SSH_FXP_WRITE
	SSH_FXP_LSTAT
	SSH_FXP_FSTAT
	SSH_FXP_SETSTAT
	SSH_FXP_FSETSTAT
	SSH_FXP_OPENDIR
	SSH_FXP_READDIR
	SSH_FXP_REMOVE
	SSH_FXP_MKDIR
	SSH_FXP_RMDIR
	SSH_FXP_REALPATH
	SSH_FXP_STAT
	SSH_FXP_RENAME
	SSH_FXP_READLINK
	SSH_FXP_LINK
	SSH_FXP_BLOCK
	SSH_FXP_UNBLOCK
)
const (
	SSH_FXP_STATUS = iota + 101
	SSH_FXP_HANDLE
	SSH_FXP_DATA
	SSH_FXP_NAME
	SSH_FXP_ATTRS
)
const (
	SSH_FXP_EXTENDED = iota + 201
	SSH_FXP_EXTENDED_REPLY
)

type Sftp struct {
	conn io.ReadWriter
	vfs  afero.Afero
	cwd  string
}

func (sftp *Sftp) GetRealPath(path string) string {
	if !pathlib.IsAbs(path) {
		path = sftp.cwd + "/" + path
	}
	return pathlib.Clean(path)
}

func NewSftp(conn io.ReadWriter, vfs afero.Fs, user string) *Sftp {
	u := honeyos.GetUser(user)
	fs := afero.Afero{vfs}
	if exists, _ := fs.DirExists(u.Homedir); !exists {
		fs.MkdirAll(u.Homedir, 0600)
	}
	return &Sftp{conn, fs, u.Homedir}
}

func (sftp *Sftp) HandleRequest() {
	for {
		req, err := readRequest(sftp.conn)
		if err != nil {
			break
		}
		switch req.Type {
		case SSH_FXP_INIT:
			sendReply(sftp.conn, createInit())
		case SSH_FXP_REALPATH:
			path := byteToStr(req.Payload)
			path = sftp.GetRealPath(path)
			fi, err := sftp.vfs.Fs.Stat(path)
			if err != nil {
				sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_BAD_MESSAGE))
				continue
			} else {
				b, err := createNamePacket([]string{path}, []os.FileInfo{fi})
				if err != nil {
					sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_BAD_MESSAGE))
					continue
				}
				sendReply(sftp.conn, sftpMsg{
					Type:    SSH_FXP_NAME,
					ReqID:   req.ReqID,
					Payload: b,
					Length:  uint32(len(b) + 1),
				})
			}
		default:
			sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_OP_UNSUPPORTED))
		}

	}
}

func readRequest(r io.Reader) (sftpMsg, error) {
	b := make([]byte, 4)
	if _, err := io.ReadFull(r, b); err != nil {
		return sftpMsg{}, err
	}
	l := binary.BigEndian.Uint32(b)
	b = make([]byte, l)
	if _, err := io.ReadFull(r, b); err != nil {
		return sftpMsg{}, err
	}
	log.Info(b)
	rplyMsg := sftpMsg{
		Length: l,
		Type:   ReqType(b[0]),
	}
	if b[0] == SSH_FXP_INIT {
		rplyMsg.Payload = b[1:]
	} else {
		rplyMsg.ReqID = binary.BigEndian.Uint32(b[1:])
		rplyMsg.Payload = b[5:]
	}
	return rplyMsg, nil
}

func sendReply(w io.Writer, reply sftpMsg) {
	size := 4 + reply.Length
	b := make([]byte, size)
	binary.BigEndian.PutUint32(b, reply.Length)
	b[4] = byte(reply.Type)
	if reply.ReqID > 0 {
		binary.BigEndian.PutUint32(b[5:], reply.ReqID)
		copy(b[9:], reply.Payload)
	} else {
		copy(b[5:], reply.Payload)
	}
	log.Infof("data to send:%v", b)
	w.Write(b)
}

func getLsString(fi os.FileInfo) string {
	uName := ""
	gName := ""

	switch p := fi.Sys().(type) {
	case virtualfs.ZipExtraInfo:
		uName = honeyos.GetUserByID(p.UID()).Name
		gName = honeyos.GetGroupByID(p.GID()).Name
	default:
		uName = "root"
		gName = "root"
	}

	size := fi.Size()
	if fi.IsDir() {
		size = 4096
	}
	return fmt.Sprintf("%v    1 %-8s %-8s %8d %v %v", fi.Mode(), uName, gName,
		size, fi.ModTime().Format("Jan 02 15:04"), fi.Name())
}

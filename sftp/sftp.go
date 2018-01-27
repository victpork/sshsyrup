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
	Type    PacketType
	ReqID   uint32
	Payload []byte
}

type Sftp struct {
	conn io.ReadWriter
	vfs  afero.Afero
	cwd  string
	quit chan<- int
}

func (sftp *Sftp) GetRealPath(path string) string {
	if !pathlib.IsAbs(path) {
		path = sftp.cwd + "/" + path
	}
	return pathlib.Clean(path)
}

func NewSftp(conn io.ReadWriter, vfs afero.Fs, user string, quitSig chan<- int) *Sftp {
	u := honeyos.GetUser(user)
	fs := afero.Afero{vfs}
	if exists, _ := fs.DirExists(u.Homedir); !exists {
		fs.MkdirAll(u.Homedir, 0600)
	}
	return &Sftp{conn, fs, u.Homedir, quitSig}
}

func (sftp *Sftp) HandleRequest() {
	for {
		req, err := readRequest(sftp.conn)
		if err != nil {
			if err == io.EOF {
				defer func() { sftp.quit <- 0 }()
			} else {
				defer func() { sftp.quit <- 1 }()
			}
			break
		}
		log.Infof("Req Rcv'd: %v\nPayload: %v", req.Type, req.Payload)
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
	if size, err := io.ReadFull(r, b); err != nil || size < 4 {
		return sftpMsg{}, err
	}
	l := binary.BigEndian.Uint32(b)
	b = make([]byte, l)
	if _, err := io.ReadFull(r, b); err != nil {
		return sftpMsg{}, err
	}
	rplyMsg := sftpMsg{
		Length: l,
		Type:   PacketType(b[0]),
	}
	if PacketType(b[0]) == SSH_FXP_INIT {
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

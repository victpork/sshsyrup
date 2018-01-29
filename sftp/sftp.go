package sftp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"

	pathlib "path"

	honeyos "github.com/mkishere/sshsyrup/os"
	"github.com/mkishere/sshsyrup/virtualfs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

type sftpMsg struct {
	Type    PacketType
	ReqID   uint32
	Payload []byte
}

type Sftp struct {
	conn          io.ReadWriter
	vfs           afero.Afero
	cwd           string
	quit          chan<- int
	fileHandleMap map[int]afero.File
	nextHandle    int
	lock          sync.RWMutex
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
	return &Sftp{conn, fs, u.Homedir, quitSig, map[int]afero.File{}, 0, sync.RWMutex{}}
}

func (sftp *Sftp) HandleRequest() {
	defer sftp.cleanUp()
	for {
		req, err := readRequest(sftp.conn)
		if err != nil {
			// Other side has disconnect, signal channel level to close
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
				})
			}
		case SSH_FXP_OPENDIR:
			path := byteToStr(req.Payload)
			if len(path) == 0 {
				sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_BAD_MESSAGE))
				continue
			}
			path = sftp.GetRealPath(path)
			fileHn, err := sftp.Open(path)
			if err != nil {
				sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_NO_SUCH_FILE))
				continue
			}
			b := make([]byte, 4+len(fileHn))
			strToByte(b, fileHn)
			sendReply(sftp.conn, sftpMsg{
				ReqID:   req.ReqID,
				Type:    SSH_FXP_HANDLE,
				Payload: b,
			})
		case SSH_FXP_READDIR:
			handle := byteToStr(req.Payload)
			b, err := sftp.readDir(handle)
			if err != nil {
				sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_FAILURE))
			}
			sendReply(sftp.conn, sftpMsg{
				Type:    SSH_FXP_NAME,
				ReqID:   req.ReqID,
				Payload: b,
			})
		case SSH_FXP_CLOSE:
			handle := byteToStr(req.Payload)
			err := sftp.close(handle)
			if err != nil {
				sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_FAILURE))
			}
			sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_OK))
		default:
			sendReply(sftp.conn, createStatusMsg(req.ReqID, SSH_FX_BAD_MESSAGE))
		}

	}
}

func (sftp *Sftp) Open(path string) (string, error) {
	sftp.lock.Lock()
	defer sftp.lock.Unlock()
	file, err := sftp.vfs.Open(path)

	if err != nil {
		return "", err
	}
	hnd := sftp.nextHandle
	sftp.fileHandleMap[hnd] = file
	sftp.nextHandle++
	return strconv.Itoa(hnd), nil
}
func (sftp *Sftp) close(hnd string) error {
	sftp.lock.Lock()
	defer sftp.lock.Unlock()
	hndInt, err := strconv.Atoi(hnd)
	if err != nil {
		return err
	}
	file, exists := sftp.fileHandleMap[hndInt]
	if !exists {
		return os.ErrNotExist
	}
	err = file.Close()
	if err != nil {
		return err
	}

	delete(sftp.fileHandleMap, hndInt)

	return nil
}
func (sftp *Sftp) readDir(hnd string) ([]byte, error) {
	sftp.lock.RLock()
	defer sftp.lock.RUnlock()
	hndInt, err := strconv.Atoi(hnd)
	if err != nil {
		return nil, err
	}
	file, exists := sftp.fileHandleMap[hndInt]
	if !exists {
		return nil, os.ErrNotExist
	}
	fi, err := file.Readdir(-1)
	if err != nil {
		return nil, err
	}
	return createNamePacket(nil, fi)
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
		Type: PacketType(b[0]),
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
	payloadLen := uint32(len(reply.Payload) + 1)
	if reply.ReqID > 0 {
		payloadLen += 4
	}
	b := make([]byte, payloadLen+4)
	binary.BigEndian.PutUint32(b, payloadLen)
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
	uid, gid, _, _ := virtualfs.GetExtraInfo(fi)
	uName := honeyos.GetUserByID(uid).Name
	gName := honeyos.GetGroupByID(gid).Name

	size := fi.Size()
	if fi.IsDir() {
		size = 4096
	}
	return fmt.Sprintf("%v    1 %-8s %-8s %8d %v %v", fi.Mode(), uName, gName,
		size, fi.ModTime().Format("Jan 02 15:04"), fi.Name())
}

func (sftp *Sftp) cleanUp() {
	if len(sftp.fileHandleMap) > 0 {
		for _, file := range sftp.fileHandleMap {
			file.Close()
		}
	}

	if er := recover(); er != nil {
		log.Error("Recover from parsing error")
	}

}

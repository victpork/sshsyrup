package sftp

import (
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"time"

	"github.com/mkishere/sshsyrup/virtualfs"
)

type fxp_realpath struct {
	OrigPath string
}

type fxp_name []virtualfs.FileInfo

type PacketType byte

const (
	SSH_FXP_INIT PacketType = iota + 1
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
	SSH_FXP_STATUS PacketType = iota + 101
	SSH_FXP_HANDLE
	SSH_FXP_DATA
	SSH_FXP_NAME
	SSH_FXP_ATTRS
)
const (
	SSH_FXP_EXTENDED PacketType = iota + 201
	SSH_FXP_EXTENDED_REPLY
)

type AttrFlag uint32

const (
	SSH_FILEXFER_ATTR_SIZE AttrFlag = 1 << iota
	SSH_FILEXFER_ATTR_UIDGID
	SSH_FILEXFER_ATTR_PERMISSIONS
	SSH_FILEXFER_ATTR_ACMODTIME
	SSH_FILEXFER_ATTR_EXTENDED AttrFlag = 0x80000000
)

type StatusCode uint32

const (
	SSH_FX_OK StatusCode = iota
	SSH_FX_EOF
	SSH_FX_NO_SUCH_FILE
	SSH_FX_PERMISSION_DENIED
	SSH_FX_FAILURE
	SSH_FX_BAD_MESSAGE
	SSH_FX_NO_CONNECTION
	SSH_FX_CONNECTION_LOST
	SSH_FX_OP_UNSUPPORTED
)

func ToByte(data interface{}) (b []byte) {
	switch data.(type) {
	case fxp_name:
		fiArray := data.(fxp_name)
		b = make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(len(fiArray)))

	}
	return
}

func strToByte(b []byte, s string) {
	_ = b[3+len(s)]
	binary.BigEndian.PutUint32(b, uint32(len(s)))
	copy(b[4:], []byte(s))
}

func byteToStr(b []byte) string {
	return string(b[4:])
}

func fileAttrToByte(b []byte, fi os.FileInfo) {
	var uid, gid uint32
	var atime, mtime time.Time
	switch realFInfo := fi.Sys().(type) {
	case virtualfs.ZipExtraInfo:
		uid = uint32(realFInfo.UID())
		gid = uint32(realFInfo.GID())
		atime = realFInfo.Atime()
		mtime = realFInfo.Mtime()
	default:
		uid = 0
		gid = 0
		atime = time.Now()
		mtime = time.Now()
	}
	_ = b[35]
	b[3] = 32
	// attributes variable struct, and also variable per protocol version
	// spec version 3 attributes:
	// uint32   flags
	// uint64   size           present only if flag SSH_FILEXFER_ATTR_SIZE
	// uint32   uid            present only if flag SSH_FILEXFER_ATTR_UIDGID
	// uint32   gid            present only if flag SSH_FILEXFER_ATTR_UIDGID
	// uint32   permissions    present only if flag SSH_FILEXFER_ATTR_PERMISSIONS
	// uint32   atime          present only if flag SSH_FILEXFER_ACMODTIME
	// uint32   mtime          present only if flag SSH_FILEXFER_ACMODTIME
	// uint32   extended_count present only if flag SSH_FILEXFER_ATTR_EXTENDED
	// string   extended_type
	// string   extended_data
	// ...      more extended data (extended_type - extended_data pairs),
	// 	   so that number of pairs equals extended_count
	binary.BigEndian.PutUint32(b[4:], uint32(SSH_FILEXFER_ATTR_SIZE|
		SSH_FILEXFER_ATTR_UIDGID|
		SSH_FILEXFER_ATTR_PERMISSIONS|
		SSH_FILEXFER_ATTR_ACMODTIME))

	if fi.IsDir() {
		binary.BigEndian.PutUint64(b[8:], 4096)
	} else {
		binary.BigEndian.PutUint64(b[8:], uint64(fi.Size()))
	}
	binary.BigEndian.PutUint32(b[16:], uint32(uid))
	binary.BigEndian.PutUint32(b[20:], uint32(gid))
	binary.BigEndian.PutUint32(b[24:], fileModeToBit(fi.Mode()))
	binary.BigEndian.PutUint32(b[28:], uint32(atime.Unix()))
	binary.BigEndian.PutUint32(b[32:], uint32(mtime.Unix()))
}

func createInit() sftpMsg {
	payload := make([]byte, 94)
	payload[3] = 3
	strToByte(payload[4:], "posix-rename@openssh.com")
	strToByte(payload[32:], "1")
	strToByte(payload[37:], "statvfs@openssh.com")
	strToByte(payload[60:], "2")
	strToByte(payload[65:], "fstatvfs@openssh.com")
	strToByte(payload[89:], "2")

	return sftpMsg{
		Type:    SSH_FXP_VERSION,
		Payload: payload,
	}
}

func createNamePacket(names []string, fileInfo []os.FileInfo) ([]byte, error) {
	if names == nil {
		names = make([]string, len(fileInfo))
		for i, fi := range fileInfo {
			names[i] = fi.Name()
		}
	} else if len(names) != len(fileInfo) {
		return nil, errors.New("name and fileinfo does not match")
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(names)))
	for i := range names {
		fileInfoB := make([]byte, len(names[i])*2+4+36+55)
		strToByte(fileInfoB, names[i])
		strToByte(fileInfoB[4+len(names[i]):], getLsString(fileInfo[i]))
		fileAttrToByte(fileInfoB[4+len(names[i])*2+36:], fileInfo[i])
		b = append(b, fileInfoB...)
	}

	return b, nil
}

func createStatusMsg(reqID uint32, statusCode StatusCode) sftpMsg {
	stsMsgs := []string{
		"Success",           /* SSH_FX_OK */
		"End of file",       /* SSH_FX_EOF */
		"No such file",      /* SSH_FX_NO_SUCH_FILE */
		"Permission denied", /* SSH_FX_PERMISSION_DENIED */
		"Failure",           /* SSH_FX_FAILURE */
		"Bad message",       /* SSH_FX_BAD_MESSAGE */
		"No connection",     /* SSH_FX_NO_CONNECTION */
		"Connection lost",   /* SSH_FX_CONNECTION_LOST */
		"Operation unsupported",
	}
	stsMsg := stsMsgs[statusCode]
	strBuf := make([]byte, len(stsMsg)+8)
	binary.BigEndian.PutUint32(strBuf, uint32(statusCode))
	strToByte(strBuf[4:], stsMsg)
	msg := sftpMsg{
		Type:    SSH_FXP_STATUS,
		ReqID:   reqID,
		Payload: strBuf,
	}
	return msg
}

// fromFileMode converts from the os.FileMode specification to sftp filemode bits
// Copied from https://github.com/pkg/sftp/
func fileModeToBit(mode os.FileMode) uint32 {
	ret := uint32(0)

	if mode&os.ModeDevice != 0 {
		if mode&os.ModeCharDevice != 0 {
			ret |= syscall.S_IFCHR
		} else {
			ret |= syscall.S_IFBLK
		}
	}
	if mode&os.ModeDir != 0 {
		ret |= syscall.S_IFDIR
	}
	if mode&os.ModeSymlink != 0 {
		ret |= syscall.S_IFLNK
	}
	if mode&os.ModeNamedPipe != 0 {
		ret |= syscall.S_IFIFO
	}
	if mode&os.ModeSetgid != 0 {
		ret |= syscall.S_ISGID
	}
	if mode&os.ModeSetuid != 0 {
		ret |= syscall.S_ISUID
	}
	if mode&os.ModeSticky != 0 {
		ret |= syscall.S_ISVTX
	}
	if mode&os.ModeSocket != 0 {
		ret |= syscall.S_IFSOCK
	}

	if mode&os.ModeType == 0 {
		ret |= syscall.S_IFREG
	}
	ret |= uint32(mode & os.ModePerm)

	return ret
}

package sftp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mkishere/sshsyrup/virtualfs"
)

type fxp_realpath struct {
	OrigPath string
}

type fxp_name []virtualfs.FileInfo

const (
	SSH_FILEXFER_ATTR_SIZE              = 0x00000001
	SSH_FILEXFER_ATTR_PERMISSIONS       = 0x00000004
	SSH_FILEXFER_ATTR_ACCESSTIME        = 0x00000008
	SSH_FILEXFER_ATTR_CREATETIME        = 0x00000010
	SSH_FILEXFER_ATTR_MODIFYTIME        = 0x00000020
	SSH_FILEXFER_ATTR_ACL               = 0x00000040
	SSH_FILEXFER_ATTR_OWNERGROUP        = 0x00000080
	SSH_FILEXFER_ATTR_SUBSECOND_TIMES   = 0x00000100
	SSH_FILEXFER_ATTR_BITS              = 0x00000200
	SSH_FILEXFER_ATTR_ALLOCATION_SIZE   = 0x00000400
	SSH_FILEXFER_ATTR_TEXT_HINT         = 0x00000800
	SSH_FILEXFER_ATTR_MIME_TYPE         = 0x00001000
	SSH_FILEXFER_ATTR_LINK_COUNT        = 0x00002000
	SSH_FILEXFER_ATTR_UNTRANSLATED_NAME = 0x00004000
	SSH_FILEXFER_ATTR_CTIME             = 0x00008000
	SSH_FILEXFER_ATTR_EXTENDED          = 0x80000000
)

const (
	OK_TEXT        = "Success"               /* SSH_FX_OK */
	EOF_TEXT       = "End of file"           /* SSH_FX_EOF */
	NO_FILE_TEXT   = "No such file"          /* SSH_FX_NO_SUCH_FILE */
	PERM_TEXT      = "Permission denied"     /* SSH_FX_PERMISSION_DENIED */
	FAIL_TEXT      = "Failure"               /* SSH_FX_FAILURE */
	BADMSG_TEXT    = "Bad message"           /* SSH_FX_BAD_MESSAGE */
	NO_CONN_TEXT   = "No connection"         /* SSH_FX_NO_CONNECTION */
	CONN_LOST_TEXT = "Connection lost"       /* SSH_FX_CONNECTION_LOST */
	OP_UNSUP_TEXT  = "Operation unsupported" /* SSH_FX_OP_UNSUPPORTED */
	UNKNOWN_TEXT   = "Unknown error"         /* Others */
)

const (
	SSH_FX_OK = iota
	SSH_FX_EOF
	SSH_FX_NO_SUCH_FILE
	SSH_FX_PERMISSION_DENIED
	SSH_FX_FAILURE
	SSH_FX_BAD_MESSAGE
	SSH_FX_NO_CONNECTION
	SSH_FX_CONNECTION_LOST
	SSH_FX_OP_UNSUPPORTED
	SSH_FX_INVALID_HANDLE
	SSH_FX_NO_SUCH_PATH
	SSH_FX_FILE_ALREADY_EXISTS
	SSH_FX_WRITE_PROTECT
	SSH_FX_NO_MEDIA
	SSH_FX_NO_SPACE_ON_FILESYSTEM
	SSH_FX_QUOTA_EXCEEDED
	SSH_FX_UNKNOWN_PRINCIPAL
	SSH_FX_LOCK_CONFLICT
	SSH_FX_DIR_NOT_EMPTY
	SSH_FX_NOT_A_DIRECTORY
	SSH_FX_INVALID_FILENAME
	SSH_FX_LINK_LOOP
	SSH_FX_CANNOT_DELETE
	SSH_FX_INVALID_PARAMETER
	SSH_FX_FILE_IS_A_DIRECTORY
	SSH_FX_BYTE_RANGE_LOCK_CONFLICT
	SSH_FX_BYTE_RANGE_LOCK_REFUSED
	SSH_FX_DELETE_PENDING
	SSH_FX_FILE_CORRUPT
	SSH_FX_OWNER_INVALID
	SSH_FX_GROUP_INVALID
	SSH_FX_NO_MATCHING_BYTE_RANGE_LOCK
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
	binary.BigEndian.PutUint32(b[4:], SSH_FILEXFER_ATTR_SIZE|
		SSH_FILEXFER_ATTR_PERMISSIONS|
		SSH_FILEXFER_ATTR_MODIFYTIME)

	if fi.IsDir() {
		binary.BigEndian.PutUint64(b[8:], 4096)
	} else {
		binary.BigEndian.PutUint64(b[8:], uint64(fi.Size()))
	}
	fmt.Printf("After size: %v\n", b)
	binary.BigEndian.PutUint32(b[16:], uint32(uid))
	binary.BigEndian.PutUint32(b[20:], uint32(gid))
	fmt.Printf("After UID/GID: %v\n", b)
	binary.BigEndian.PutUint32(b[24:], uint32(fi.Mode()))
	fmt.Printf("After Mode: %v\n", b)
	binary.BigEndian.PutUint32(b[28:], uint32(atime.Unix()))
	fmt.Printf("After Atime: %v\n", b)
	binary.BigEndian.PutUint32(b[32:], uint32(mtime.Unix()))
	fmt.Printf("After MTime: %v\n", b)
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
		Length:  uint32(len(payload) + 1),
		Payload: payload,
	}
}

func createNamePacket(names []string, fileInfo []os.FileInfo) ([]byte, error) {
	if len(names) != len(fileInfo) {
		return nil, errors.New("name and fileinfo does not match")
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(len(names)))
	for i := range names {
		fileInfoB := make([]byte, len(names[i])*2+4+36+55)
		strToByte(fileInfoB, names[i])
		strToByte(fileInfoB, getLsString(fileInfo[i]))
		fileAttrToByte(fileInfoB[4+len(names[i]):], fileInfo[i])
		b = append(b, fileInfoB...)
	}

	return b, nil
}

func createStatusMsg(reqID, statusCode uint32) sftpMsg {
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
		"Unknown error",
	}
	var stsMsg string
	if statusCode > 9 {
		stsMsg = stsMsgs[9]
	} else {
		stsMsg = stsMsgs[statusCode]
	}
	strBuf := make([]byte, len(stsMsg)+4)
	strToByte(strBuf, stsMsg)
	msg := sftpMsg{
		Type:    SSH_FXP_STATUS,
		ReqID:   reqID,
		Payload: strBuf,
		Length:  uint32(len(strBuf) + 1),
	}
	return msg
}

package virtualfs

import (
	"archive/zip"
	"encoding/binary"
	"os"
	"time"
)

type FileInfo struct {
	os.FileInfo
	extraInfo ZipExtraInfo
}

type ZipExtraInfo struct {
	zh    *zip.FileHeader
	ctime time.Time
	atime time.Time
	mtime time.Time
	uid   int
	gid   int
}

type unixFileInfo struct {
	UID int
	GID int
}

type unixTimestampInfo struct {
	ModTime time.Time
	AccTime time.Time
	CreTime time.Time
}

func (fi FileInfo) Sys() interface{} {
	zipHeader := fi.FileInfo.Sys().(*zip.FileHeader)

	unixInfo, ts := readExtraHeader(zipHeader.Extra)
	finfo := ZipExtraInfo{
		zh:    fi.FileInfo.Sys().(*zip.FileHeader),
		ctime: ts.CreTime,
		atime: ts.AccTime,
		mtime: ts.ModTime,
		uid:   unixInfo.UID,
		gid:   unixInfo.GID,
	}
	return finfo
}

func readExtraHeader(dataField []byte) (fileInfo unixFileInfo, tsInfo unixTimestampInfo) {
	for pos := 0; pos < len(dataField); {
		fieldID := binary.LittleEndian.Uint16(dataField[pos : pos+2])
		pos += 2
		fieldLen := binary.LittleEndian.Uint16(dataField[pos : pos+2])
		pos += 2
		switch fieldID {
		case 0x5455: // Modification timestamp
			// Referenced from https://github.com/koron/go-zipext/blob/master/zipext.go
			tsInfo = unixTimestampInfo{}
			flag := dataField[pos]
			pos++
			if flag&0x01 != 0 && len(dataField)-pos >= 4 {
				tsInfo.ModTime = time.Unix(int64(binary.LittleEndian.Uint32(dataField[pos:])), 0)
				pos += 4
			}
			if flag&0x02 != 0 && len(dataField)-pos >= 4 && fieldLen > 5 {
				tsInfo.AccTime = time.Unix(int64(binary.LittleEndian.Uint32(dataField[pos:])), 0)
				pos += 4
			}
			if flag&0x04 != 0 && len(dataField)-pos >= 4 && fieldLen > 9 {
				tsInfo.CreTime = time.Unix(int64(binary.LittleEndian.Uint32(dataField[pos:])), 0)
				pos += 4
			}
		case 0x7875: // UNIX UID/GID
			fileInfo = unixFileInfo{}
			pos++ // skip version field
			uidLen := int(dataField[pos])
			pos++
			fileInfo.UID = int(readVariableInt(dataField[pos : pos+uidLen]))
			pos += uidLen
			gidLen := int(dataField[pos])
			pos++
			fileInfo.GID = int(readVariableInt(dataField[pos : pos+gidLen]))
			pos += gidLen
		default:
			//Skip the whole field
			pos += int(fieldLen)
		}

	}
	return
}

func readVariableInt(field []byte) uint32 {
	switch len(field) {
	case 4:
		return binary.LittleEndian.Uint32(field)
	case 8:
		return uint32(binary.LittleEndian.Uint64(field))
	}
	return 0
}

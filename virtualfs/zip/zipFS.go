package zip

import (
	"archive/zip"
	"encoding/binary"
	"time"

	"github.com/mkishere/sshsyrup/virtualfs"
)

type unixFileInfo struct {
	UID int
	GID int
}

type unixTimestampInfo struct {
	ModTime time.Time
	AccTime time.Time
	CreTime time.Time
}

func readExtraHeader(dataField []byte) (fileInfo *unixFileInfo) {

	for pos := 0; pos < len(dataField); {
		fieldID := binary.LittleEndian.Uint16(dataField[pos : pos+2])
		pos += 2
		fieldLen := binary.LittleEndian.Uint16(dataField[pos : pos+2])
		pos += 2
		switch fieldID {
		case 0x7875: // UNIX UID/GID
			fileInfo = &unixFileInfo{}
			pos++ // skip version field
			uidLen := int(dataField[pos+1])
			pos++
			fileInfo.UID = int(readVariableInt(dataField[pos : pos+uidLen]))
			pos += uidLen
			gidLen := int(dataField[pos+1])
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

// CreateZipFS creates a VFS based on the zip file provided in argument
func CreateZipFS(file string, uidMap, gidMap map[int]string) (*virtualfs.VirtualFS, error) {
	r, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	vfs := virtualfs.NewFS()
	// Start to build index
	for _, f := range r.File {
		fileInfo := readExtraHeader(f.FileHeader.Extra)
		filePath := "/" + f.Name
		if f.FileInfo().IsDir() {
			vfs.Mkdir(filePath, uidMap[fileInfo.UID], gidMap[fileInfo.GID], f.FileInfo().Mode())
		} else {
			if f.UncompressedSize64 > 0 {
				r, err := f.Open()
				if err != nil {
					// Skip if error
					continue
				}
				vfs.Mkfile(filePath, uidMap[fileInfo.UID], gidMap[fileInfo.GID], f.FileInfo().Mode(), &r)
			} else {
				vfs.Mkfile(filePath, uidMap[fileInfo.UID], gidMap[fileInfo.GID], f.FileInfo().Mode(), nil)
			}
		}
	}
	return vfs, nil
}

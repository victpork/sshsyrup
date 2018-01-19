package main

import (
	"archive/zip"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type zeroSizefileInfo struct {
	fi os.FileInfo
}

var (
	zipFile   string
	dir       string
	stripData bool
	skip      string
)

func (z zeroSizefileInfo) Sys() interface{}   { return z.fi.Sys() }
func (z zeroSizefileInfo) Size() int64        { return 0 }
func (z zeroSizefileInfo) IsDir() bool        { return z.fi.IsDir() }
func (z zeroSizefileInfo) Name() string       { return z.fi.Name() }
func (z zeroSizefileInfo) Mode() os.FileMode  { return z.fi.Mode() }
func (z zeroSizefileInfo) ModTime() time.Time { return z.fi.ModTime() }

func init() {
	flag.StringVar(&zipFile, "o", "", "Output zip file")
	flag.StringVar(&dir, "p", "", "Starting position for the import path")
	flag.BoolVar(&stripData, "b", true, "Strip file content, if set to true the program will only read metadata from filesystem and skip actual file content in archive.")
	flag.StringVar(&skip, "k", "", "Paths to be skipped for indexing, separated by semicolons")

}

func writeExtraUnixInfo(uid, gid uint32) (b []byte) {
	b = make([]byte, 15)
	binary.LittleEndian.PutUint16(b, 0x7875)
	binary.LittleEndian.PutUint16(b[2:], 11)
	b[4] = 1
	b[5] = 4
	binary.LittleEndian.PutUint32(b[6:], uid)
	b[10] = 4
	binary.LittleEndian.PutUint32(b[11:], gid)
	return
}

func main() {
	flag.Parse()
	if len(dir) == 0 {
		fmt.Println("Missing parameter -p. See -help")
		return
	}
	if len(zipFile) == 0 {
		fmt.Println("Missing parameter -o. See -help")
		return
	}
	skipPath := []string{}
	if len(skip) > 0 {
		skipPath = strings.Split(skip, ";")
	}
	f, err := os.OpenFile(zipFile, os.O_CREATE|os.O_WRONLY, os.ModeExclusive)
	if err != nil {
		if os.IsExist(err) {
			fmt.Println("File already exists")
		} else {
			fmt.Printf("Cannot create file. Reason:%v", err)
		}
		return
	}
	defer f.Close()
	archive := zip.NewWriter(f)
	defer archive.Close()

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if path == dir {
			return nil
		}
		for _, v := range skipPath {
			if strings.HasPrefix(path, v) {
				return nil
			}
		}
		if info == nil {
			fmt.Printf("Skipping %v for nil FileInfo\n", path)
			return nil
		}
		fmt.Printf("Writing %v (%v)\n", path, info.Name())
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if stripData && !info.IsDir() {
			info = zeroSizefileInfo{fi: info}
		}
		header, err := zip.FileInfoHeader(info)
		header.Name = strings.TrimPrefix(path, dir+"/")
		header.Name = strings.TrimPrefix(path, "/")
		header.Extra = writeExtraUnixInfo(getExtraInfo(info))
		fmt.Printf("Filename to be written:%v\n", header.Name)
		if err != nil {
			fmt.Println(err)
			return err
		}
		if info.IsDir() {
			header.Name += "/"
		} else if info.Size() > 0 || info.Mode()&os.ModeSymlink != 0 {
			header.Method = zip.Deflate
		}

		filepath.Base(dir)
		w, err := archive.CreateHeader(header)

		if err != nil {
			fmt.Println(err)
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			dst, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(dst))
			if err != nil {
				return err
			}
		} else if !stripData {
			file, err := os.Open(path)
			if err != nil {
				fmt.Println(err)
				return err
			}
			defer file.Close()
			_, err = io.Copy(w, file)
			fmt.Println(err)
			return err
		}

		return nil
	})
	err = archive.Close()
	if err != nil {
		fmt.Println(err)
	}
}

package virtualfs

// Mostly referenced from https://github.com/hillu/afero
import (
	"archive/zip"
	"io"
	"os"
	"syscall"

	"github.com/spf13/afero"
)

type File struct {
	os.FileInfo
	zipFile  *zip.File
	children map[string]*File
	reader   io.ReadCloser
	SymLink  string
	closed   bool
	offset   int64
	buf      []byte
}

func (f *File) fillBuffer(offset int64) (err error) {
	if f.reader == nil {
		if f.reader, err = f.zipFile.Open(); err != nil {
			return
		}
	}
	if offset > int64(f.zipFile.UncompressedSize64) {
		offset = int64(f.zipFile.UncompressedSize64)
		err = io.EOF
	}
	if len(f.buf) >= int(offset) {
		return
	}
	buf := make([]byte, int(offset)-len(f.buf))
	n, _ := io.ReadFull(f.reader, buf)
	if n > 0 {
		f.buf = append(f.buf, buf[:n]...)
	}
	return
}

func (f *File) Close() (err error) {
	f.zipFile = nil
	f.closed = true
	f.buf = nil
	if f.reader != nil {
		err = f.reader.Close()
		f.reader = nil
	}
	return
}

func (f *File) Read(p []byte) (n int, err error) {
	if f.FileInfo.IsDir() {
		return 0, syscall.EISDIR
	}
	if f.closed {
		return 0, afero.ErrFileClosed
	}
	err = f.fillBuffer(f.offset + int64(len(p)))
	n = copy(p, f.buf[f.offset:])
	f.offset += int64(len(p))
	return
}

func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	if f.FileInfo.IsDir() {
		return 0, syscall.EISDIR
	}
	if f.closed {
		return 0, afero.ErrFileClosed
	}
	err = f.fillBuffer(off + int64(len(p)))
	n = copy(p, f.buf[int(off):])
	return
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.FileInfo.IsDir() {
		return 0, syscall.EISDIR
	}
	if f.closed {
		return 0, afero.ErrFileClosed
	}
	switch whence {
	case os.SEEK_SET:
	case os.SEEK_CUR:
		offset += f.offset
	case os.SEEK_END:
		offset += int64(f.zipFile.UncompressedSize64)
	default:
		return 0, syscall.EINVAL
	}
	if offset < 0 || offset > int64(f.zipFile.UncompressedSize64) {
		return 0, afero.ErrOutOfRange
	}
	f.offset = offset
	return offset, nil
}

func (f *File) Write(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}
func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, os.ErrPermission
}

/* func (f *File) Name() string {
	return f.
} */

func (f *File) Readdir(n int) ([]os.FileInfo, error) {
	m := f.children
	nArr := make([]os.FileInfo, 0, len(m))
	for _, node := range m {
		nArr = append(nArr, node)
	}
	if n <= 0 || n >= len(m) {
		return nArr, nil
	}
	return nArr[:n], nil
}
func (f *File) Readdirnames(n int) ([]string, error) {
	m := f.children
	nArr := make([]string, 0, len(m))
	for name, _ := range m {
		nArr = append(nArr, name)
	}
	if n <= 0 || n >= len(m) {
		return nArr, nil
	}
	return nArr[:n], nil
}
func (f *File) Stat() (os.FileInfo, error) {
	return f, nil
}
func (f *File) Sync() error {
	return nil
}
func (f *File) Truncate(size int64) error {
	return os.ErrPermission
}
func (f *File) WriteString(s string) (ret int, err error) {
	return 0, os.ErrPermission
}

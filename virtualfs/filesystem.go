package virtualfs

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	pathlib "path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

type VirtualFS struct {
	root *File
}

type rootInfo struct{}

func (rootInfo) Name() string       { return string(filepath.Separator) }
func (rootInfo) Size() int64        { return 0 }
func (rootInfo) Mode() os.FileMode  { return os.ModeDir | os.ModePerm }
func (rootInfo) ModTime() time.Time { return time.Now() }
func (rootInfo) IsDir() bool        { return true }
func (rootInfo) Sys() interface{}   { return nil }

// NewVirtualFS initalized the tree, which creates the root directory
func NewVirtualFS(zipFile string) (afero.Fs, error) {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	vfs := &VirtualFS{
		root: &File{
			FileInfo: rootInfo{},
			children: make(map[string]*File),
		},
	}
	for _, f := range r.File {
		vfs.createNode(f)
	}
	return vfs, nil
}

func (t *VirtualFS) Name() string {
	return "zipFS"
}

func (t *VirtualFS) createNode(f *zip.File) error {
	n := &File{
		zipFile:  f,
		FileInfo: FileInfo{FileInfo: f.FileInfo()},
	}
	if n.Mode()&os.ModeDir != 0 {
		n.children = make(map[string]*File)
	} else if n.Mode()&os.ModeSymlink != 0 {
		rd, err := f.Open()
		if err != nil {
			return err
		}
		buf := bytes.NewBuffer(nil)
		io.Copy(buf, rd)
		n.SymLink = buf.String()
		rd.Close()
	}
	dir, nodeName := pathlib.Split("/" + strings.TrimSuffix(f.Name, "/"))
	var parent *File
	var err error
	parent, err = t.fetchNode(dir, false)
	if err != nil {
		return err
	}
	parent.children[nodeName] = n

	return nil
}

// Mkdir creates a new directory according to the path argument passed in
func (t *VirtualFS) Mkdir(path string, mode os.FileMode) error {
	return &os.PathError{Op: "mkdir", Err: os.ErrPermission, Path: path}
}

// MkdirAll creates a new directory according to the path argument passed in
func (t *VirtualFS) MkdirAll(path string, mode os.FileMode) error {
	return &os.PathError{Op: "mkdir", Err: os.ErrPermission, Path: path}
}

func (t *VirtualFS) Remove(path string) error {
	return &os.PathError{Op: "remove", Err: os.ErrPermission, Path: path}
}

func (t *VirtualFS) RemoveAll(path string) error {
	return &os.PathError{Op: "remove", Err: os.ErrPermission, Path: path}
}

func (t *VirtualFS) Rename(new, old string) error {
	return &os.PathError{Op: "rename", Err: os.ErrPermission, Path: old}
}

func (t *VirtualFS) fetchNode(path string, followSymLink bool) (*File, error) {
	if strings.HasPrefix(path, "\\") {
		path = strings.Replace(path, "\\", "/", -1)
	}
	path = pathlib.Clean(path)
	cwd := t.root

	if path == "/" {
		return t.root, nil
	}
	dirs := strings.Split(path, "/")
	for _, nodeName := range dirs[1:] {
		if cwd.Mode()&os.ModeSymlink != 0 && followSymLink {
			var err error
			cwd, err = t.fetchNode(cwd.SymLink, true)
			if err != nil {
				return nil, err
			}
		}
		node, nodeExists := cwd.children[nodeName]
		if !nodeExists {
			return nil, &os.PathError{Op: "open", Err: os.ErrNotExist, Path: path}
		}
		cwd = node
	}
	return cwd, nil
}

func (t *VirtualFS) Create(path string) (afero.File, error) {
	return nil, &os.PathError{Op: "create", Err: os.ErrPermission, Path: path}
}

func (t *VirtualFS) Open(path string) (afero.File, error) {
	n, err := t.fetchNode(path, false)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (t *VirtualFS) OpenFile(path string, flag int, mode os.FileMode) (afero.File, error) {
	node, err := t.fetchNode(path, true)
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (t *VirtualFS) Stat(path string) (os.FileInfo, error) {
	n, err := t.fetchNode(path, true)
	if err != nil {
		return nil, err
	}
	return n.FileInfo, nil
}

func (t *VirtualFS) Chmod(path string, mode os.FileMode) error {
	return &os.PathError{Op: "chmod", Err: os.ErrPermission, Path: path}
}

func (t *VirtualFS) Chtimes(path string, modTime, accTime time.Time) error {
	return &os.PathError{Op: "chtimes", Err: os.ErrPermission, Path: path}
}

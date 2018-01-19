package virtualfs

import (
	"bytes"
	"errors"
	"io"
	"os"
	pathlib "path"
	"strings"
	"sync"
	"time"
)

const (
	TypeFile = iota
	TypeDir
	TypeLink
)

type Permission uint8

const (
	READ    = 1
	WRITE   = 2
	EXECUTE = 4
)

var (
	FileNotFound     = errors.New("File not found")
	NodeAlreadyExist = errors.New("Node with same name already exists")
)

type VirtualFS struct {
	root *Node
	lock sync.RWMutex
}

// Node are for describing the filesystem object
type Node struct {
	FileMode os.FileMode
	Uid      int
	Gid      int
	modTime  time.Time
	size     int64
	Children map[string]*Node
	reader   *io.ReadCloser
	writer   *io.WriteCloser
	Pointer  *Node
}

type File struct {
	fd  uint
	buf bytes.Buffer
}

// NewFS initalized the tree, which creates the root directory
func NewFS() *VirtualFS {
	return &VirtualFS{
		root: createNode(0, 0, os.ModeType),
	}
}

func createNode(uid, gid int, mode os.FileMode) (n *Node) {
	n = &Node{
		FileMode: mode,
		Uid:      uid,
		Gid:      gid,
	}
	if mode&os.ModeDir != 0 {
		n.Children = make(map[string]*Node)
	}
	return
}

func (t *VirtualFS) IsExist(path string) bool {
	_, err := t.fetchNode(path)
	if err != nil {
		return false
	}
	return true
}

// Mkdir creates a new directory according to the path argument passed in
func (t *VirtualFS) Mkdir(path string, uid, gid int, mode os.FileMode) error {
	if strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	parent, newDir := pathlib.Split(path)

	cwd, err := t.fetchNode(parent)
	if err != nil {
		return err
	}
	if cwd.FileMode&os.ModeDir == 0 {
		return os.ErrInvalid
	}
	if _, exists := cwd.Children[newDir]; exists {
		return os.ErrExist
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	newNode := createNode(uid, gid, cwd.FileMode)
	newNode.modTime = time.Now()
	newNode.size = 4096
	cwd.Children[newDir] = newNode
	newNode.Children["."] = newNode
	newNode.Children[".."] = cwd
	return nil
}

func (t *VirtualFS) fetchNode(path string) (*Node, error) {
	path = pathlib.Clean(path)
	cwd := t.root

	if path == "/" {
		return t.root, nil
	}
	dirs := strings.Split(path, "/")
	for _, nodeName := range dirs {
		if len(nodeName) == 0 {
			continue
		}
		node, nodeExists := cwd.Children[nodeName]
		if !nodeExists {
			return nil, FileNotFound
		}
		cwd = node
	}
	return cwd, nil
}

func (t *VirtualFS) ReadDir(path string) (map[string]*Node, error) {
	dir, err := t.fetchNode(path)
	if err != nil {
		return nil, err
	}
	roChildMap := make(map[string]*Node)
	for k, v := range dir.Children {
		roChildMap[k] = v
	}
	return roChildMap, nil
}

func (t *VirtualFS) Mkfile(path string, uid, gid int, mode os.FileMode, contentReader *io.ReadCloser) error {
	dirName, fileName := pathlib.Split(path)
	t.lock.Lock()
	defer t.lock.Unlock()
	dirNode, err := t.fetchNode(dirName)
	if err != nil {
		return err
	}
	if _, exists := dirNode.Children[fileName]; exists {
		return NodeAlreadyExist
	}
	newFile := createNode(uid, gid, mode)
	newFile.reader = contentReader
	newFile.modTime = time.Now()
	newFile.size = 0
	dirNode.Children[fileName] = newFile

	return nil
}

func (t *VirtualFS) MkFileWithFileInfo(path string, uid, gid int, fileInfo os.FileInfo, contentReader *io.ReadCloser) error {
	dirName, fileName := pathlib.Split(path)
	if fileName != fileInfo.Name() {
		return errors.New("Differentname")
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	dirNode, err := t.fetchNode(dirName)
	if err != nil {
		return err
	}
	if _, exists := dirNode.Children[fileName]; exists {
		return NodeAlreadyExist
	}
	newFile := createNode(uid, gid, fileInfo.Mode())
	newFile.reader = contentReader
	newFile.modTime = fileInfo.ModTime()
	newFile.size = fileInfo.Size()
	dirNode.Children[fileName] = newFile

	return nil
}

func (t *VirtualFS) OpenFile(path string, flag int, mode os.FileMode) error {
	node, err := t.fetchNode(path)
	if err != nil {
		return err
	}
	if node.FileMode&os.ModeDir != 0 {
		return os.ErrInvalid
	}
	return nil
}

func (t *VirtualFS) CopyFile(dst, src string, uid, gid int) error {
	node, err := t.fetchNode(src)
	if err != nil {
		return err
	}
	parentPath, child := pathlib.Split(dst)
	parentNode, err := t.fetchNode(parentPath)
	if err != nil {
		return err
	}
	if _, exists := parentNode.Children[child]; exists {
		return errors.New("Target already exists")
	}
	parentNode.Children[child] = createNode(uid, gid, node.FileMode)
	return nil
}

func (t *VirtualFS) MoveFile(dst, src string, uid, gid int) error {
	err := t.CopyFile(dst, src, uid, gid)
	if err != nil {
		return err
	}
	err = t.DeleteNode(src)
	if err != nil {
		return err
	}
	return nil
}

func (t *VirtualFS) DeleteNode(path string) error {
	dirName, fileName := pathlib.Split(path)
	dirNode, err := t.fetchNode(dirName)
	if err != nil {
		return err
	}
	if _, exists := dirNode.Children[fileName]; exists {
		return FileNotFound
	}
	delete(dirNode.Children, fileName)
	return nil
}

func (t *VirtualFS) Stat(path string) (os.FileInfo, error) {
	dirName, fileName := pathlib.Split(path)
	dirNode, err := t.fetchNode(dirName)
	if err != nil {
		return nil, err
	}
	childNode, exists := dirNode.Children[fileName]
	if !exists {
		return nil, FileNotFound
	}
	return &FileInfo{
		path: path,
		n:    childNode,
	}, nil
}

func (t *VirtualFS) Root() *Node { return t.root }

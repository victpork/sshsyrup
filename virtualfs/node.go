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
	FileNotFound = errors.New("File not found")
)

type VirtualFS struct {
	Root *Node
	cPos *Node
	lock sync.RWMutex
}

// Node are for describing the filesystem object
type Node struct {
	FileMode os.FileMode
	Uid      uint
	Gid      uint

	Children map[string]*Node
	Data     []byte
	Pointer  *Node
}

type File struct {
	fd  uint
	buf bytes.Buffer
}

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	sys     interface{}
}

// Init initalized the tree, which creates the root directory
func Init() *VirtualFS {
	return &VirtualFS{
		Root: createNode(os.ModeType),
	}
}

func createNode(mode os.FileMode) (n *Node) {
	n = &Node{
		FileMode: mode,
	}
	if n.IsDir() {
		n.Children = make(map[string]*Node)
	}
	return
}

// Mkdir creates a new directory according to the path argument passed in
func (t *VirtualFS) Mkdir(path string, mode os.FileMode) error {
	parent, newDir := pathlib.Split(path)
	cwd, err := t.fetchNode(parent)
	if err != nil {
		return err
	}
	if !cwd.IsDir() {
		return io.EOF
	}
	t.lock.Lock()
	defer t.lock.Unlock()
	cwd.Children[newDir] = createNode(cwd.FileMode)
	return nil
}

func (t *VirtualFS) fetchNode(path string) (*Node, error) {
	path = pathlib.Clean(path)
	var cwd *Node
	if pathlib.IsAbs(path) {
		cwd = t.Root
	} else {
		cwd = t.cPos
	}
	if path == "/" {
		return t.Root, nil
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
	return dir.Children, nil
}

func (t *VirtualFS) Mkfile(path string, mode os.FileMode, content []byte) error {
	dirName, fileName := pathlib.Split(path)
	dirNode, err := t.fetchNode(dirName)
	if err != nil {
		return err
	}
	dirNode.Children[fileName] = createNode(mode)
	copy(dirNode.Children[fileName].Data, content)

	return nil
}

func (t *VirtualFS) OpenFile(path string, mode os.FileMode) error {
	node, err := t.fetchNode(path)
	if err != nil {
		return err
	}
	if node.IsDir() {
		return errors.New("Target is directory")
	}
	return nil
}

func (t *VirtualFS) CopyFile(dst, src string) error {
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
	parentNode.Children[child] = createNode(node.FileMode)
	return nil
}

func (t *VirtualFS) MoveFile(dst, src string) error {
	err := t.CopyFile(dst, src)
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
	return childNode, nil
}

func (n *Node) IsDir() bool {
	return n.FileMode&os.ModeDir != 0
}

func (n *Node) ModTime() time.Time {
	return time.Now()
}

func (n *Node) Mode() os.FileMode {
	return n.FileMode
}

func (n *Node) Name() string {
	return ""
}

func (n *Node) Size() int64 {
	if n.IsDir() {
		return 4096
	}
	return int64(len(n.Data))
}

func (n *Node) Sys() interface{} {
	return nil
}

package virtualfs

import "testing"

func TestCreateFS(t *testing.T) {
	vfs := NewFS()
	err := vfs.Mkdir("/bin", "root", "root", 600)
	if err != nil {
		t.Error(err)
	}
	n, err := vfs.fetchNode("/bin")
	if err != nil {
		t.Error(err)
	}
	fi, err := vfs.Stat("/bin")
	if !fi.IsDir() {
		t.Error("/bin is not DIR")
	}
	if fi.Name() != "/bin" {
		t.Error("name incorrect")
	}
	if fi.Size() != 4096 {
		t.Error("Size incorrct")
	}
	err = vfs.Mkfile("/bin/ls", "root", "root", 600, nil)
	if err != nil {
		t.Error(err)
	}
	lsNode, err := vfs.fetchNode("/bin/ls")
	if err != nil {
		t.Error(err)
	}
	fi, err = vfs.Stat("/bin/ls")
	if fi.Name() != "/bin/ls" {
		t.Error("name incorrect")
	}
	if lsNode != n.Children["ls"] {
		t.Error("2 different node created")
	}
	err = vfs.Mkfile("/bin/cat", "root", "root", 600, nil)
	if err != nil {
		t.Error(err)
	}
	if len(n.Children) != 4 {
		t.Error("Child node count incorrect")
	}
}

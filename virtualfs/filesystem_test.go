package virtualfs

import "testing"

func TestCreateFS(t *testing.T) {
	vfs, err := NewVirtualFS("../filesystem.zip")
	if err != nil {
		t.Fatal(err)
	}
	bootDir, err := vfs.Open("/boot")
	if err != nil {
		t.Error(err)
	}
	dirNames, err := bootDir.Readdirnames(10)
	if err != nil {
		t.Error(err)
	}
	if len(dirNames) != 6 {
		t.Error("Dir don't match")
	}
}

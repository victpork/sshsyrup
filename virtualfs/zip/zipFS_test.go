package zip

import "testing"

func TestCreateFSFromZip(t *testing.T) {
	idmap := map[int]string{
		0: "root",
	}
	vfs, err := CreateZipFS("../../demofs.zip", idmap, idmap)
	if err != nil {
		t.Error("Error creating filesystem from zip")
		return
	}

	dir, err := vfs.ReadDir("/bin")
	if err != nil {
		t.Log(vfs.Root().Children)
		t.Log(dir)
		t.Error("Error reading directory")
	}
	if ls, ex := dir["ls"]; ex {
		if ls.Uid != "root" || ls.Gid != "root" {
			t.Error("UID/GID incorrect")
		}
	} else {
		t.Error("/bin/ls not found")
	}
}

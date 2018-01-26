package sftp

import (
	"bytes"
	"testing"

	"os"

	"github.com/mkishere/sshsyrup/virtualfs"
)

func TestStrToByte(t *testing.T) {
	b := make([]byte, 9)
	strToByte(b, "Hello")
	if bytes.Compare(b, []byte{0, 0, 0, 5, 72, 101, 108, 108, 111}) != 0 {
		t.Errorf("Result mismatch: %v", b)
	}
}

func TestInitPacket(t *testing.T) {
	p := createInit()
	if int(p.Length) != len(p.Payload)+1 {
		t.Errorf("Size mismatch: Expected %v, Actual %v", p.Length, len(p.Payload))
	}
}

func TestReplyInitPacket(t *testing.T) {
	p := createInit()
	b := bytes.Buffer{}
	sendReply(&b, p)
	if int(p.Length) != len(p.Payload)+1 {
		t.Errorf("Size mismatch: Expected %v, Actual %v", p.Length, len(p.Payload))
	}
}

func TestNamePacket(t *testing.T) {
	vfs, err := virtualfs.NewVirtualFS("../filesystem.zip")
	if err != nil {
		t.Fatal(err)
	}
	fi, err := vfs.Stat("/home/mk")
	b, err := createNamePacket([]string{"/home/mk"}, []os.FileInfo{fi})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 3 {
		t.Error(b)
	}
}
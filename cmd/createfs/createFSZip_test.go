package main

import (
	"bytes"
	"testing"
)

func TestZipExtraHeader(t *testing.T) {
	b := writeExtraUnixInfo(0, 0)
	if bytes.Compare([]byte{117, 120, 11, 00, 01, 04, 00, 00, 00, 00, 04, 00, 00, 00, 00}, b) != 0 {
		t.Errorf("Byte array issue: %v", b)
	}
}

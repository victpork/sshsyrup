package termlogger

import (
	"io"

	"github.com/sirupsen/logrus"
)

type LogHook interface {
	io.Closer
	logrus.Hook
}

// DummyWriter is a writer that discard everything that writes in
// Consider like writing into /dev/null
type DummyWriter struct{}

func (DummyWriter) Write(p []byte) (int, error) { return len(p), nil }

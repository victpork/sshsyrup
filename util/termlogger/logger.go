package termlogger

import (
	"io"

	log "github.com/sirupsen/logrus"
)

// ioLogWrapper logs terminal keystrokes
type ioLogWrapper struct {
	dataStream io.ReadWriter
	keylog     *log.Logger
	hook       LogHook
}

// NewLogger creates a Logger instance
func NewLogger(logHook LogHook, ds io.ReadWriter) io.ReadWriteCloser {
	tl := &ioLogWrapper{
		dataStream: ds,
		keylog:     log.New(),
		hook:       logHook,
	}
	tl.keylog.SetLevel(log.InfoLevel)
	tl.keylog.Out = DummyWriter{}
	tl.keylog.AddHook(logHook)
	return tl
}

func (tl *ioLogWrapper) Read(p []byte) (n int, err error) {
	n, err = tl.dataStream.Read(p)
	defer tl.keylog.WithField("dir", input).Info(string(p[:n]))
	return
}

func (tl *ioLogWrapper) Write(p []byte) (int, error) {
	defer tl.keylog.WithField("dir", output).Info(string(p))
	return tl.dataStream.Write(p)
}

func (tl *ioLogWrapper) Close() error {
	return tl.hook.Close()
}

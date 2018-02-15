package termlogger

import (
	"io"

	log "github.com/sirupsen/logrus"
)

// ioLogWrapper logs terminal keystrokes
type ioLogWrapper struct {
	keylog   *log.Logger
	hook     LogHook
	in       io.Reader
	out, err io.Writer
}

type StdIOErr interface {
	In() io.Reader
	Out() io.Writer
	Err() io.Writer
	Close() error
}

type logWriter struct {
	*log.Entry
}

func (lw logWriter) Write(p []byte) (int, error) {
	lw.Info(string(p))
	return len(p), nil
}

// NewLogger creates a Logger instance
func NewLogger(logHook LogHook, in io.Reader, out, err io.Writer) StdIOErr {
	tl := &ioLogWrapper{
		keylog: log.New(),
		hook:   logHook,
	}
	tl.keylog.SetLevel(log.InfoLevel)
	tl.keylog.Out = DummyWriter{}
	tl.keylog.AddHook(logHook)
	inLogStream := logWriter{tl.keylog.WithField("dir", input)}
	outLogStream := logWriter{tl.keylog.WithField("dir", output)}
	tl.in = io.TeeReader(in, inLogStream)
	tl.out = io.MultiWriter(out, outLogStream)
	tl.err = io.MultiWriter(err, outLogStream)
	return tl
}

func (tl *ioLogWrapper) In() io.Reader {
	return tl.in
}

func (tl *ioLogWrapper) Out() io.Writer {
	return tl.out
}

func (tl *ioLogWrapper) Err() io.Writer {
	return tl.err
}

func (tl *ioLogWrapper) Close() error {
	return tl.hook.Close()
}

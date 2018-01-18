package termlogger

import (
	"io"
	"time"
)

// TermLogger logs terminal keystrokes
type TermLogger struct {
	logChan    chan frame
	formatter  Formatter
	quit       chan struct{}
	dataStream io.ReadWriter
}

// NewLogger creates a Logger instance
func NewLogger(formatter Formatter, ds io.ReadWriter) (tl *TermLogger) {
	tl = &TermLogger{
		logChan:    make(chan frame, 10),
		quit:       make(chan struct{}),
		dataStream: ds,
		formatter:  formatter,
	}
	go func(fChan <-chan frame, quit <-chan struct{}) {
		defer tl.formatter.Close()
		for {
			select {
			case f := <-fChan:
				if len(f.Input) > 0 {
					tl.formatter.WriteLog(f)
				}
			case <-quit:
				return
			}
		}

	}(tl.logChan, tl.quit)
	return
}

func (tl *TermLogger) Read(p []byte) (n int, err error) {
	n, err = tl.dataStream.Read(p)
	defer tl.formatter.WriteLog(frame{
		Time:  time.Now(),
		Type:  input,
		Input: p[:n],
	})
	return
}

func (tl *TermLogger) Write(p []byte) (int, error) {
	defer tl.formatter.WriteLog(frame{
		Time:  time.Now(),
		Type:  output,
		Input: p,
	})
	return tl.dataStream.Write(p)
}

// Close signals the channel and the channel listener (formatter)
// that the session ends and is going to close
func (tl *TermLogger) Close() {
	close(tl.logChan)
	close(tl.quit)
}

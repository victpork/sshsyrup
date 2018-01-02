package termlogger

import (
	"io"
)

// Logger interface for different format loggers
type Logger interface {
	io.ReadWriter
	Close()
}

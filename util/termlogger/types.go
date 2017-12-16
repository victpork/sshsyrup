package termlogger

// Logger interface for different format loggers
type Logger interface {
	Write(data []byte, direction TTYDirection) (err error)
}

package termlogger

import (
	"io"

	"github.com/sirupsen/logrus"
)

type LogHook interface {
	io.Closer
	logrus.Hook
}

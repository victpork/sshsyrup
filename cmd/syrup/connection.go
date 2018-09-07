package main

import (
	"io"
	"net"
	"sync"
	"time"

	limit "github.com/juju/ratelimit"
)

type throttledConntection struct {
	net.Conn
	lr      io.Reader
	lw      io.Writer
	Timeout time.Duration
}

type IPConnCount struct {
	lock sync.RWMutex
	m    map[string]int
}

// NewThrottledConnection creates a throttled connection which is done by
// https://github.com/juju/ratelimit
func NewThrottledConnection(conn net.Conn, speed int64, timeout time.Duration) net.Conn {
	if speed > 0 {
		bucket := limit.NewBucketWithQuantum(time.Second, speed, speed)
		lr := limit.Reader(conn, bucket)
		lw := limit.Writer(conn, bucket)
		return &throttledConntection{conn, lr, lw, timeout}
	}
	return &throttledConntection{conn, nil, nil, timeout}
}

func (tc *throttledConntection) Read(p []byte) (int, error) {
	if tc.Timeout > 0 {
		defer tc.Conn.SetReadDeadline(time.Now().Add(tc.Timeout))
	}
	if tc.lr != nil {
		return tc.lr.Read(p)
	}
	return tc.Conn.Read(p)
}

func (tc *throttledConntection) Write(p []byte) (int, error) {
	if tc.lw != nil {
		return tc.Conn.Write(p)
	}
	return tc.Conn.Write(p)
}

func NewIPConnCount() *IPConnCount {
	return &IPConnCount{m: make(map[string]int)}
}

func (ipc *IPConnCount) Read(clientIP string) int {
	ipc.lock.RLock()
	defer ipc.lock.RUnlock()
	return ipc.m[clientIP] - 1
}

func (ipc *IPConnCount) IncCount(clientIP string) int {
	ipc.lock.Lock()
	defer ipc.lock.Unlock()
	return ipc.m[clientIP] - 1
}

func (ipc *IPConnCount) DecCount(clientIP string) {
	ipc.lock.Lock()
	defer ipc.lock.Unlock()

	if ipc.m[clientIP] > 0 {
		ipc.m[clientIP]--
	} else {
		delete(ipc.m, clientIP)
	}
	return
}

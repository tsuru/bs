package log

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
)

var bufferConnSize = 4096

type bufferedConn struct {
	net.Conn
	mu      sync.Mutex
	w       *bufio.Writer
	to      time.Duration
	latency time.Duration
	done    chan struct{}
}

func newBufferedConn(conn net.Conn, maxLatency time.Duration) *bufferedConn {
	bConn := &bufferedConn{
		Conn:    conn,
		w:       bufio.NewWriterSize(conn, bufferConnSize),
		latency: maxLatency,
		done:    make(chan struct{}),
	}
	if bConn.latency > 0 {
		go bConn.flushLoop()
	}
	return bConn
}

func (c *bufferedConn) Write(msg []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.to > 0 && len(msg) > c.w.Available() {
		if err := c.Conn.SetWriteDeadline(time.Now().Add(c.to)); err != nil {
			return 0, err
		}
	}
	return c.w.Write(msg)
}

func (c *bufferedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.flush()
	close(c.done)
	closeErr := c.Conn.Close()
	if closeErr != nil {
		if err != nil {
			err = fmt.Errorf("flush error: %v, close error: %v", err, closeErr)
		} else {
			err = closeErr
		}
	}
	return err
}

func (c *bufferedConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if deadline.IsZero() {
		c.to = 0
		return c.Conn.SetWriteDeadline(time.Time{})
	} else {
		c.to = time.Until(deadline)
	}
	return nil
}

func (c *bufferedConn) flush() error {
	if c.to > 0 {
		if err := c.Conn.SetWriteDeadline(time.Now().Add(c.to)); err != nil {
			return err
		}
	}
	return c.w.Flush()
}

func (c *bufferedConn) flushLoop() {
	t := time.NewTicker(c.latency)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			c.mu.Lock()
			c.flush()
			c.mu.Unlock()
		}
	}
}

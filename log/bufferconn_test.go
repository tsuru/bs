package log

import (
	"bytes"
	"io"
	"net"
	"time"

	"gopkg.in/check.v1"
)

func listener(c *check.C) (net.Conn, chan string) {
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	ch := make(chan string)
	go func() {
		defer close(ch)
		buffer := make([]byte, 100)
		conn, err := tcpListener.Accept()
		c.Assert(err, check.IsNil)
		defer conn.Close()
		for {
			n, err := conn.Read(buffer)
			if err == io.EOF {
				return
			}
			c.Assert(err, check.IsNil)
			ch <- string(buffer[:n])
		}
	}()
	conn, _ := net.Dial("tcp", tcpListener.Addr().String())
	return conn, ch
}

func (s *S) TestBufferedConn(c *check.C) {
	conn, ch := listener(c)
	bConn := newBufferedConn(conn, 0)
	_, err := bConn.Write([]byte("msg"))
	c.Assert(err, check.IsNil)
	err = bConn.Close()
	c.Assert(err, check.IsNil)
	c.Assert(<-ch, check.Equals, "msg")
}

func (s *S) TestBufferedConnFlushOnMax(c *check.C) {
	conn, ch := listener(c)
	bConn := newBufferedConn(conn, 0)
	_, err := bConn.Write(bytes.Repeat([]byte("a"), bufferConnSize))
	c.Assert(err, check.IsNil)
	_, err = bConn.Write([]byte("b"))
	c.Assert(err, check.IsNil)
	for total := 0; total < bufferConnSize; {
		total += len(<-ch)
	}
	err = bConn.Close()
	c.Assert(err, check.IsNil)
	c.Assert(<-ch, check.Equals, "b")
}

func (s *S) TestBufferedConnMaxLatency(c *check.C) {
	conn, ch := listener(c)
	bConn := newBufferedConn(conn, time.Second)
	_, err := bConn.Write([]byte("msg"))
	c.Assert(err, check.IsNil)
	c.Assert(<-ch, check.Equals, "msg")
	err = bConn.Close()
	c.Assert(err, check.IsNil)
	c.Assert(<-ch, check.Equals, "")
}

func (s *S) TestBufferedConnDelaySetWriteDeadline(c *check.C) {
	conn, ch := listener(c)
	bConn := newBufferedConn(conn, 0)
	err := bConn.SetWriteDeadline(time.Now().Add(time.Second))
	c.Assert(err, check.IsNil)
	c.Assert(time.Second-bConn.to < time.Millisecond, check.Equals, true)
	_, err = bConn.Write([]byte("a"))
	c.Assert(err, check.IsNil)
	time.Sleep(2 * time.Second)
	_, err = bConn.Write([]byte("b"))
	c.Assert(err, check.IsNil)
	time.Sleep(time.Second)
	err = bConn.Close()
	c.Assert(err, check.IsNil)
	c.Assert(<-ch, check.Equals, "ab")
}

func (s *S) TestBufferedConnSetWriteDeadlineIsEffective(c *check.C) {
	tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	conn, err := net.Dial("tcp", tcpListener.Addr().String())
	c.Assert(err, check.IsNil)
	bConn := newBufferedConn(conn, 0)
	err = bConn.SetWriteDeadline(time.Now().Add(time.Second))
	c.Assert(err, check.IsNil)
	c.Assert(time.Second-bConn.to < time.Millisecond, check.Equals, true)
	_, err = bConn.Write([]byte("abc"))
	c.Assert(err, check.IsNil)
	bConn.flush()
	bConn.to = 0
	time.Sleep(time.Second + time.Millisecond)
	_, err = bConn.Write([]byte("abc"))
	c.Assert(err, check.IsNil)
	err = bConn.Close()
	c.Assert(err, check.ErrorMatches, `.*i/o timeout.*`)
}

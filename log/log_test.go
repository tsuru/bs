// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package log

import (
	"net"
	"testing"

	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (S) TestLogForwarderStart(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"udp://" + udpConn.LocalAddr().String()},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer func() {
		func() {
			defer func() {
				recover()
			}()
			lf.server.Kill()
		}()
		lf.server.Wait()
	}()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte("<30>2015-06-05T16:13:47Z myhost mytag: mymsg\n")
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(buffer[:n], check.DeepEquals, msg)
}

func (S) TestLogForwarderStartBindError(c *check.C) {
	lf := LogForwarder{
		BindAddress: "xudp://0.0.0.0:59317",
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `invalid protocol "xudp", expected tcp or udp`)
}

func (S) TestLogForwarderForwardConnError(c *check.C) {
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"xudp://127.0.0.1:1234"},
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `unable to connect to "xudp://127.0.0.1:1234": dial xudp: unknown network xudp`)
	lf = LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"tcp://localhost:99999"},
	}
	err = lf.Start()
	c.Assert(err, check.ErrorMatches, `unable to connect to "tcp://localhost:99999": dial tcp: invalid port 99999`)
}

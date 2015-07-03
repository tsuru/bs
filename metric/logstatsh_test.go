// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"net"

	"gopkg.in/check.v1"
)

func (S) TestSend(c *check.C) {
	addr := net.UDPAddr{
		Port: 0,
		IP:   net.ParseIP("127.0.0.1"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	c.Assert(err, check.IsNil)
	defer conn.Close()
	host, port, err := net.SplitHostPort(conn.LocalAddr().String())
	c.Assert(err, check.IsNil)
	st := logStash{
		Client: "teste",
		Host:   host,
		Port:   port,
	}
	err = st.Send("key", "value")
	c.Assert(err, check.IsNil)
}

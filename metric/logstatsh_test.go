// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"
	"net"

	"gopkg.in/check.v1"
)

func (s *S) TestSend(c *check.C) {
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
		Client: "test",
		Host:   host,
		Port:   port,
	}
	err = st.Send("app", "hostname", "process", "key", "value")
	c.Assert(err, check.IsNil)
	var data [246]byte
	n, _, err := conn.ReadFrom(data[:])
	c.Assert(err, check.IsNil)
	expected := map[string]interface{}{
		"count":   float64(1),
		"client":  "test",
		"metric":  "key",
		"value":   "value",
		"app":     "app",
		"host":    "hostname",
		"process": "process",
	}
	var got map[string]interface{}
	err = json.Unmarshal(data[:n], &got)
	c.Assert(err, check.IsNil)
	c.Assert(got, check.DeepEquals, expected)
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"
	"net"
	"os"

	"gopkg.in/check.v1"
)

func (s *S) TestSend(c *check.C) {
	addr := net.UDPAddr{IP: net.ParseIP("127.0.0.1")}
	conn, err := net.ListenUDP("udp", &addr)
	c.Assert(err, check.IsNil)
	defer conn.Close()
	host, port, err := net.SplitHostPort(conn.LocalAddr().String())
	c.Assert(err, check.IsNil)
	st := logStash{
		Client:   "test",
		Host:     host,
		Port:     port,
		Protocol: "udp",
	}
	err = st.Send(ContainerInfo{app: "app", hostname: "hostname", process: "process"}, "key", "value")
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
	err = st.Send(ContainerInfo{name: "container", hostname: "hostname", image: "image"}, "key", "value")
	c.Assert(err, check.IsNil)
	n, _, err = conn.ReadFrom(data[:])
	c.Assert(err, check.IsNil)
	expected = map[string]interface{}{
		"count":     float64(1),
		"client":    "test",
		"metric":    "key",
		"value":     "value",
		"host":      "hostname",
		"image":     "image",
		"container": "container",
	}
	got = make(map[string]interface{})
	err = json.Unmarshal(data[:n], &got)
	c.Assert(err, check.IsNil)
	c.Assert(got, check.DeepEquals, expected)
}

func (s *S) TestSendTCP(c *check.C) {
	addr := net.TCPAddr{IP: net.ParseIP("127.0.0.1")}
	conn, err := net.ListenTCP("tcp", &addr)
	c.Assert(err, check.IsNil)
	defer conn.Close()
	dataCh := make(chan []byte, 1)

	go func() {
		client, innerErr := conn.Accept()
		c.Assert(innerErr, check.IsNil)
		defer client.Close()
		var data [264]byte
		n, innerErr := client.Read(data[:])
		c.Assert(innerErr, check.IsNil)
		dataCh <- data[:n]
	}()

	host, port, err := net.SplitHostPort(conn.Addr().String())
	c.Assert(err, check.IsNil)
	st := logStash{
		Client:   "test",
		Host:     host,
		Port:     port,
		Protocol: "tcp",
	}
	err = st.Send(ContainerInfo{app: "app", hostname: "hostname", process: "process"}, "key", "value")
	c.Assert(err, check.IsNil)
	data := <-dataCh
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
	err = json.Unmarshal(data, &got)
	c.Assert(err, check.IsNil)
	c.Assert(got, check.DeepEquals, expected)
}

func (s *S) TestNewLogStasDefaults(c *check.C) {
	os.Unsetenv("METRICS_LOGSTASH_CLIENT")
	os.Unsetenv("METRICS_LOGSTASH_HOST")
	os.Unsetenv("METRICS_LOGSTASH_PORT")
	os.Unsetenv("METRICS_LOGSTASH_PROTOCOL")

	st, err := newLogStash()
	c.Assert(err, check.IsNil)
	expected := &logStash{
		Host:     "localhost",
		Port:     "1984",
		Client:   "tsuru",
		Protocol: "udp",
	}
	c.Assert(st, check.DeepEquals, expected)
}

func (s *S) TestNewLogStashEnvs(c *check.C) {
	os.Setenv("METRICS_LOGSTASH_CLIENT", "tsurutest")
	os.Setenv("METRICS_LOGSTASH_HOST", "127.0.0.1")
	os.Setenv("METRICS_LOGSTASH_PORT", "1983")
	os.Setenv("METRICS_LOGSTASH_PROTOCOL", "tcp")

	st, err := newLogStash()
	c.Assert(err, check.IsNil)
	expected := &logStash{
		Host:     "127.0.0.1",
		Port:     "1983",
		Client:   "tsurutest",
		Protocol: "tcp",
	}
	c.Assert(st, check.DeepEquals, expected)
}

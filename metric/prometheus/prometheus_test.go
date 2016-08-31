// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"testing"

	"github.com/tsuru/bs/metric"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (s *S) TestSend(c *check.C) {
	st := prometheus{}
	err := st.Send(metric.ContainerInfo{
		App:      "app",
		Hostname: "hostname",
		Process:  "process",
	}, "key", "value")
	c.Assert(err, check.IsNil)
}

func (s *S) TestSendConn(c *check.C) {
	st := prometheus{}
	err := st.SendConn(metric.ContainerInfo{
		App:      "app",
		Hostname: "hostname",
		Process:  "process",
	}, "host")
	c.Assert(err, check.IsNil)
}

func (s *S) TestSendHost(c *check.C) {
	st := prometheus{}
	err := st.SendHost(metric.HostInfo{}, "key", "value")
	c.Assert(err, check.IsNil)
}

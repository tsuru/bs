// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"bytes"
	"log"
	"os"

	"github.com/tsuru/commandmocker"
	"gopkg.in/check.v1"
)

func (*S) TestConntrack(c *check.C) {
	dir, err := commandmocker.Add("conntrack", conntrackXML)
	c.Assert(err, check.IsNil)
	defer commandmocker.Remove(dir)
	conns, err := conntrack()
	c.Assert(err, check.IsNil)
	expected := []conn{
		{Source: "192.168.50.4:33404", Destination: "192.168.50.4:2375"},
		{Source: "172.17.42.1:42418", Destination: "172.17.0.2:4001"},
		{Source: "172.17.42.1:42428", Destination: "172.17.0.2:4001"},
		{Source: "192.168.50.4:53922", Destination: "192.168.50.4:5000"},
		{Source: "192.168.50.4:43227", Destination: "192.168.50.4:8080"},
		{Source: "172.17.0.27:39502", Destination: "172.17.42.1:4001"},
		{Source: "192.168.50.4:33496", Destination: "192.168.50.4:2375"},
		{Source: "192.168.50.4:33495", Destination: "192.168.50.4:2375"},
		{Source: "10.211.55.2:51388", Destination: "10.211.55.184:22"},
		{Source: "172.17.0.27:39492", Destination: "172.17.42.1:4001"},
		{Source: "10.211.55.2:51370", Destination: "10.211.55.184:22"},
	}
	c.Assert(conns, check.DeepEquals, expected)
}

func (*S) TestConntrackCommandFailure(c *check.C) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	dir, err := commandmocker.Error("conntrack", "something went wrong", 120)
	c.Assert(err, check.IsNil)
	defer commandmocker.Remove(dir)
	conns, err := conntrack()
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "exit status 120")
	c.Assert(buf.String(), check.Matches, "(?m).* conntrack failed: exit status 120. Output: something went wrong$")
	c.Assert(conns, check.IsNil)
}

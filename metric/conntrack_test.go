// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"github.com/tsuru/commandmocker"
	"gopkg.in/check.v1"
)

func (*S) TestConntrack(c *check.C) {
	dir, err := commandmocker.Add("conntrack", conntrackXML)
	c.Assert(err, check.IsNil)
	defer commandmocker.Remove(dir) //nolint
	conns, err := conntrack()
	c.Assert(err, check.IsNil)
	expected := []conn{
		{SourceIP: "192.168.50.4", SourcePort: "33404", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "172.17.42.1", SourcePort: "42418", DestinationIP: "172.17.0.2", DestinationPort: "4001"},
		{SourceIP: "172.17.42.1", SourcePort: "42428", DestinationIP: "172.17.0.2", DestinationPort: "4001"},
		{SourceIP: "192.168.50.4", SourcePort: "53922", DestinationIP: "192.168.50.4", DestinationPort: "5000"},
		{SourceIP: "192.168.50.4", SourcePort: "43227", DestinationIP: "192.168.50.4", DestinationPort: "8080"},
		{SourceIP: "172.17.0.27", SourcePort: "39502", DestinationIP: "172.17.42.1", DestinationPort: "4001"},
		{SourceIP: "192.168.50.4", SourcePort: "33496", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "192.168.50.4", SourcePort: "33495", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "10.211.55.2", SourcePort: "51388", DestinationIP: "10.211.55.184", DestinationPort: "22"},
		{SourceIP: "172.17.0.27", SourcePort: "39492", DestinationIP: "172.17.42.1", DestinationPort: "4001"},
		{SourceIP: "10.211.55.2", SourcePort: "51370", DestinationIP: "10.211.55.184", DestinationPort: "22"},
	}
	c.Assert(conns, check.DeepEquals, expected)
}

func (*S) TestConntrackCommandFailure(c *check.C) {
	dir, err := commandmocker.Error("conntrack", "something went wrong", 120)
	c.Assert(err, check.IsNil)
	defer commandmocker.Remove(dir) //nolint
	conns, err := conntrack()
	c.Assert(err, check.ErrorMatches, "conntrack failed: exit status 120. Output: something went wrong")
	c.Assert(conns, check.IsNil)
}

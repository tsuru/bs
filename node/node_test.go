// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package node

import (
	"testing"

	"gopkg.in/check.v1"
)

var _ = check.Suite(&H{})

type H struct{}

func Test(t *testing.T) {
	check.TestingT(t)
}

func (h *H) TestGetNodeAddrs(c *check.C) {
	addrs, err := GetNodeAddrs()
	c.Assert(err, check.IsNil)
	c.Assert(addrs, check.NotNil)
}

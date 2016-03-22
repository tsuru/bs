// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
    "gopkg.in/check.v1"
)

func (s *S) TestGetSystemLoad(c *check.C) {
    load, err := getSystemLoad()
    
    c.Assert(err, check.IsNil)
    c.Assert(load["load1"], check.Not(check.Equals), float(0))
    c.Assert(load["load5"], check.Not(check.Equals), float(0))
    c.Assert(load["load15"], check.Not(check.Equals), float(0))
}

func (s *S) TestGetHostname(c *check.C) {
    hostname, err := getHostname()
    
    c.Assert(err, check.IsNil)
    c.Assert(hostname, check.NotNil)
}
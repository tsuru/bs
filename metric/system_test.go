// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
    "gopkg.in/check.v1"
)

func (s *S) TestGetSystemMetrics(c *check.C) {
    metrics, err := getSystemMetrics()
    c.Assert(err, check.IsNil)
    s.assertLoad(c, metrics[0])
    s.assertMem(c, metrics[1])
    s.assertSwap(c, metrics[2])
}

func (s *S) TestGetSystemLoad(c *check.C) {
    load, err := getSystemLoad()
    c.Assert(err, check.IsNil)
    s.assertLoad(c, load)
}

func (s *S) TestGetSystemMem(c *check.C) {
    mem, err := getSystemMem()
    c.Assert(err, check.IsNil)
    s.assertMem(c, mem)
}

func (s *S) TestGetHostname(c *check.C) {
    hostname, err := getHostname()
    c.Assert(err, check.IsNil)
    c.Assert(hostname, check.NotNil)
}

func (s *S) TestGetSwap(c *check.C) {
    swap, err := getSystemSwap()
    c.Assert(err, check.IsNil)
    s.assertSwap(c, swap)
}

func (s *S) assertLoad(c *check.C, load map[string]float) {
    c.Assert(load["load1"], check.Not(check.Equals), float(0))
    c.Assert(load["load5"], check.Not(check.Equals), float(0))
    c.Assert(load["load15"], check.Not(check.Equals), float(0))
}

func (s *S) assertMem(c *check.C, mem map[string]float) {
    c.Assert(mem["mem_total"], check.Not(check.Equals), float(0))
    c.Assert(mem["mem_used"], check.Not(check.Equals), float(0))
    c.Assert(mem["mem_free"], check.Not(check.Equals), float(0))
}

func (s *S) assertSwap(c *check.C, swap map[string]float) {
    c.Assert(swap["swap_total"], check.Not(check.Equals), float(0))
    c.Assert(swap["swap_used"], check.Not(check.Equals), float(0))
    c.Assert(swap["swap_free"], check.Not(check.Equals), float(0))
}
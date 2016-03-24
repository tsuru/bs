// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"gopkg.in/check.v1"
	"os"
)

var _ = check.Suite(&H{})

type H struct{}

func (h *H) SetUpTest(c *check.C) {
	os.Setenv("HOST_PROC", "/proc")
}

func (h *H) TestNewHostClient(c *check.C) {
	hostClient, err := NewHostClient()
	c.Assert(err, check.IsNil)
	c.Assert(hostClient, check.NotNil)
}

func (h *H) TestNewHostClientFail(c *check.C) {
	os.Setenv("HOST_PROC", "")
	hostClient, err := NewHostClient()
	c.Assert(err, check.NotNil)
	c.Assert(hostClient, check.IsNil)
}

func (h *H) TestGetSystemMetrics(c *check.C) {
	hostClient, _ := NewHostClient()
	metrics, err := hostClient.GetHostMetrics()
	c.Assert(err, check.IsNil)
	h.assertLoad(c, metrics[0])
	h.assertMem(c, metrics[1])
	h.assertSwap(c, metrics[2])
	h.assertFileSystem(c, metrics[3])
	h.assertUptime(c, metrics[4])
	h.assertCpuTimes(c, metrics[5])
	h.assertNetworkUsage(c, metrics[6])
}

func (h *H) TestGetSystemLoad(c *check.C) {
	hostClient, _ := NewHostClient()
	load, err := hostClient.getHostLoad()
	c.Assert(err, check.IsNil)
	h.assertLoad(c, load)
}

func (h *H) TestGetSystemMem(c *check.C) {
	hostClient, _ := NewHostClient()
	mem, err := hostClient.getHostMem()
	c.Assert(err, check.IsNil)
	h.assertMem(c, mem)
}

func (h *H) TestGetHostname(c *check.C) {
	hostClient, _ := NewHostClient()
	hostname, err := hostClient.GetHostname()
	c.Assert(err, check.IsNil)
	c.Assert(hostname, check.NotNil)
}

func (h *H) TestGetSwap(c *check.C) {
	hostClient, _ := NewHostClient()
	swap, err := hostClient.getHostSwap()
	c.Assert(err, check.IsNil)
	h.assertSwap(c, swap)
}

func (h *H) TestGetFileSystemUsage(c *check.C) {
	hostClient, _ := NewHostClient()
	disk, err := hostClient.getHostFileSystemUsage()
	c.Assert(err, check.IsNil)
	h.assertFileSystem(c, disk)
}

func (h *H) TestGetUptime(c *check.C) {
	hostClient, _ := NewHostClient()
	uptime, err := hostClient.getHostUptime()
	c.Assert(err, check.IsNil)
	h.assertUptime(c, uptime)
}

func (h *H) TestGetCpuTimes(c *check.C) {
	hostClient, _ := NewHostClient()
	cpu, err := hostClient.getHostCpuTimes()
	c.Assert(err, check.IsNil)
	h.assertCpuTimes(c, cpu)
}

func (h *H) TestGetHostNetworkUsage(c *check.C) {
	hostClient, _ := NewHostClient()
	net, err := hostClient.getHostNetworkUsage()
	c.Assert(err, check.IsNil)
	h.assertNetworkUsage(c, net)
}

func (h *H) assertNetworkUsage(c *check.C, net map[string]float) {
	c.Assert(net["bytes_recv"], check.Not(check.Equals), float(0))
	c.Assert(net["bytes_sent"], check.Not(check.Equals), float(0))
}

func (h *H) assertCpuTimes(c *check.C, cpu map[string]float) {
	c.Assert(cpu["cpu_user"], check.Not(check.Equals), float(0))
	c.Assert(cpu["cpu_sys"], check.Not(check.Equals), float(0))
	c.Assert(cpu["cpu_idle"], check.Not(check.Equals), float(0))
}

func (h *H) assertLoad(c *check.C, load map[string]float) {
	c.Assert(load["load1"], check.Not(check.Equals), float(0))
	c.Assert(load["load5"], check.Not(check.Equals), float(0))
	c.Assert(load["load15"], check.Not(check.Equals), float(0))
}

func (h *H) assertMem(c *check.C, mem map[string]float) {
	c.Assert(mem["mem_total"], check.Not(check.Equals), float(0))
	c.Assert(mem["mem_used"], check.Not(check.Equals), float(0))
	c.Assert(mem["mem_free"], check.Not(check.Equals), float(0))
}

func (h *H) assertSwap(c *check.C, swap map[string]float) {
	c.Assert(swap, check.HasLen, 3)
}

func (h *H) assertFileSystem(c *check.C, disk map[string]float) {
	c.Assert(disk["disk_total"], check.Not(check.Equals), float(0))
	c.Assert(disk["disk_used"], check.Not(check.Equals), float(0))
	c.Assert(disk["disk_free"], check.Not(check.Equals), float(0))
}

func (h *H) assertUptime(c *check.C, uptime map[string]float) {
	c.Assert(uptime["uptime"], check.Not(check.Equals), float(0))
}

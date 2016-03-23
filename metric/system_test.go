// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"github.com/elastic/gosigar"
	"gopkg.in/check.v1"
	"time"
)

type FakeSigar struct{}

func (f *FakeSigar) CollectCpuStats(collectionInterval time.Duration) (<-chan gosigar.Cpu, chan<- struct{}) {
	return nil, nil
}

func (f *FakeSigar) GetFileSystemUsage(string) (gosigar.FileSystemUsage, error) {
	return gosigar.FileSystemUsage{Total: 300, Used: 200, Free: 100}, nil
}

func (f *FakeSigar) GetLoadAverage() (gosigar.LoadAverage, error) {
	return gosigar.LoadAverage{One: 1.2, Five: 2.3, Fifteen: 0.8}, nil
}

func (f *FakeSigar) GetMem() (gosigar.Mem, error) {
	return gosigar.Mem{Total: 1000, Used: 800, Free: 200}, nil
}

func (f *FakeSigar) GetSwap() (gosigar.Swap, error) {
	return gosigar.Swap{Total: 100, Free: 20, Used: 80}, nil
}

func (s *S) TestGetSystemMetrics(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	metrics, err := info.GetSystemMetrics()
	c.Assert(err, check.IsNil)
	s.assertLoad(c, metrics[0])
	s.assertMem(c, metrics[1])
	s.assertSwap(c, metrics[2])
	s.assertFileSystem(c, metrics[3])
	s.assertUptime(c, metrics[4])
}

func (s *S) TestGetSystemLoad(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	load, err := info.getSystemLoad()
	c.Assert(err, check.IsNil)
	s.assertLoad(c, load)
}

func (s *S) TestGetSystemMem(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	mem, err := info.getSystemMem()
	c.Assert(err, check.IsNil)
	s.assertMem(c, mem)
}

func (s *S) TestGetHostname(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	hostname, err := info.GetHostname()
	c.Assert(err, check.IsNil)
	c.Assert(hostname, check.NotNil)
}

func (s *S) TestGetSwap(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	swap, err := info.getSystemSwap()
	c.Assert(err, check.IsNil)
	s.assertSwap(c, swap)
}

func (s *S) TestGetFileSystemUsage(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	disk, err := info.getFileSystemUsage()
	c.Assert(err, check.IsNil)
	s.assertFileSystem(c, disk)
}

func (s *S) TestGetUptime(c *check.C) {
	fakeSigar := &FakeSigar{}
	info := SysInfo{sigar: fakeSigar}

	uptime, err := info.getUptime()
	c.Assert(err, check.IsNil)
	s.assertUptime(c, uptime)
}

func (s *S) assertLoad(c *check.C, load map[string]float) {
	c.Assert(load["load1"], check.Equals, float(1.2))
	c.Assert(load["load5"], check.Equals, float(2.3))
	c.Assert(load["load15"], check.Equals, float(0.8))
}

func (s *S) assertMem(c *check.C, mem map[string]float) {
	c.Assert(mem["mem_total"], check.Equals, float(1000))
	c.Assert(mem["mem_used"], check.Equals, float(800))
	c.Assert(mem["mem_free"], check.Equals, float(200))
}

func (s *S) assertSwap(c *check.C, swap map[string]float) {
	c.Assert(swap["swap_total"], check.Equals, float(100))
	c.Assert(swap["swap_used"], check.Equals, float(80))
	c.Assert(swap["swap_free"], check.Equals, float(20))
}

func (s *S) assertFileSystem(c *check.C, disk map[string]float) {
	c.Assert(disk["disk_total"], check.Equals, float(300))
	c.Assert(disk["disk_used"], check.Equals, float(200))
	c.Assert(disk["disk_free"], check.Equals, float(100))
}

func (s *S) assertUptime(c *check.C, uptime map[string]float) {
	c.Assert(uptime["uptime"], check.Not(check.Equals), float(0))
}

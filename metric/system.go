// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"github.com/elastic/gosigar"
	"os"
)

type SysInfo struct {
	sigar gosigar.Sigar
}

func NewSysInfo() *SysInfo {
	return &SysInfo{sigar: &gosigar.ConcreteSigar{}}
}

func (s *SysInfo) GetSystemMetrics() ([]map[string]float, error) {
	collectors := []func() (map[string]float, error){
		s.getSystemLoad,
		s.getSystemMem,
		s.getSystemSwap,
		s.getFileSystemUsage,
		s.getUptime,
		s.getCpuTimes,
	}
	var metrics []map[string]float
	for _, collector := range collectors {
		metric, err := collector()
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func (s *SysInfo) getSystemLoad() (map[string]float, error) {
	load, err := s.sigar.GetLoadAverage()
	if err != nil {
		return nil, err
	}
	stats := map[string]float{
		"load1":  float(load.One),
		"load5":  float(load.Five),
		"load15": float(load.Fifteen),
	}
	return stats, nil
}

func (s *SysInfo) getSystemMem() (map[string]float, error) {
	mem, err := s.sigar.GetMem()
	if err != nil {
		return nil, err
	}
	stats := map[string]float{
		"mem_total": float(mem.Total),
		"mem_used":  float(mem.Used),
		"mem_free":  float(mem.Free),
	}
	return stats, nil
}

func (s *SysInfo) getSystemSwap() (map[string]float, error) {
	swap, err := s.sigar.GetSwap()
	if err != nil {
		return nil, err
	}
	stats := map[string]float{
		"swap_total": float(swap.Total),
		"swap_used":  float(swap.Used),
		"swap_free":  float(swap.Free),
	}
	return stats, nil
}

func (s *SysInfo) getFileSystemUsage() (map[string]float, error) {
	disk, err := s.sigar.GetFileSystemUsage("/")
	if err != nil {
		return nil, err
	}
	stats := map[string]float{
		"disk_total": float(disk.Total),
		"disk_used":  float(disk.Used),
		"disk_free":  float(disk.Free),
	}
	return stats, nil
}

func (s *SysInfo) getUptime() (map[string]float, error) {
	uptime := gosigar.Uptime{}
	err := uptime.Get()
	if err != nil {
		return nil, err
	}
	stats := map[string]float{"uptime": float(uptime.Length)}
	return stats, nil
}

func (s *SysInfo) getCpuTimes() (map[string]float, error) {
	cpu := gosigar.Cpu{}
	err := cpu.Get()
	if err != nil {
		return nil, err
	}
	cpuTotal := float(cpu.Total())

	stats := map[string]float{
		"cpu_user":   float(cpu.User) / cpuTotal,
		"cpu_sys":    float(cpu.Sys) / cpuTotal,
		"cpu_idle":   float(cpu.Idle) / cpuTotal,
		"cpu_stolen": float(cpu.Stolen) / cpuTotal,
	}
	return stats, nil
}

func (s *SysInfo) GetHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname, nil
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"strconv"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/container"
)

func statsToMetricsMap(c *container.Container, conns []conn) (map[string]string, error) {
	s, err := c.Stats()
	if err != nil {
		return nil, err
	}
	previousCPU := s.PreCPUStats.CPUUsage.TotalUsage
	previousSystem := s.PreCPUStats.SystemCPUUsage
	cpuPercent := calculateCPUPercent(previousCPU, previousSystem, s)
	memPercent := float64(s.MemoryStats.Usage) / float64(s.MemoryStats.Limit) * 100.0
	stats := map[string]string{
		"cpu_max":     strconv.FormatFloat(cpuPercent, 'f', 2, 64),
		"mem_max":     strconv.FormatUint(s.MemoryStats.Usage, 10),
		"mem_pct_max": strconv.FormatFloat(memPercent, 'f', 2, 64),
	}
	return stats, nil
}

func calculateCPUPercent(previousCPU, previousSystem uint64, s *docker.Stats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(s.CPUStats.CPUUsage.TotalUsage - previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(s.CPUStats.SystemCPUUsage - previousSystem)
	)
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(s.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

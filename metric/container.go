// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type container struct {
	Config *docker.Config
	ID     string
}

func (c *container) metricEnabled() bool {
	for _, val := range c.Config.Env {
		if strings.HasPrefix(val, "TSURU_METRICS_BACKEND") {
			return true
		}
	}
	return false
}

func (c *container) metrics(dockerEndpoint string) (map[string]string, error) {
	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return nil, err
	}
	statsC := make(chan *docker.Stats)
	opts := docker.StatsOptions{
		ID:     c.ID,
		Stream: false,
		Stats:  statsC,
	}
	go func() {
		err := client.Stats(opts)
		if err != nil {
			log.Printf("[ERROR] cannot get stats for container %q: %s", c, err)
			return
		}
	}()
	s := <-statsC
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

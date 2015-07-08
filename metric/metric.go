// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

func reportMetrics(dockerEndpoint string) {
	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return
	}
	containers, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		log.Printf("[ERROR] failed to list containers in the Docker server at %q: %s", dockerEndpoint, err)
		return
	}
	getMetrics(dockerEndpoint, containers)
}

func metricsEnabled(container *docker.Container) bool {
	for _, val := range container.Config.Env {
		if strings.HasPrefix(val, "TSURU_METRICS_BACKEND") {
			return true
		}
	}
	return false
}

func getMetrics(dockerEndpoint string, containers []docker.APIContainers) {
	var wg sync.WaitGroup
	for _, container := range containers {
		wg.Add(1)
		go func(c docker.APIContainers) {
			defer wg.Done()
			client, err := docker.NewClient(dockerEndpoint)
			if err != nil {
				log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
				return
			}
			container, err := client.InspectContainer(c.ID)
			if err != nil {
				log.Printf("[ERROR] cannot inspect container %q dockerclient instance: %s", container, err)
				return
			}
			if !metricsEnabled(container) {
				log.Printf("[INFO] metrics not enabled for container %q. Skipping.", container.ID)
				return
			}
			metrics, err := getMetricFromContainer(dockerEndpoint, container)
			if err != nil {
				log.Printf("[ERROR] failed to get metrics for container %q in the Docker server at %q: %s", container, dockerEndpoint, err)
				return
			}
			err = sendMetrics(container, metrics)
			if err != nil {
				log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			}
		}(container)
	}
	wg.Wait()
}

func getMetricFromContainer(dockerEndpoint string, container *docker.Container) (map[string]string, error) {
	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return nil, err
	}
	statsC := make(chan *docker.Stats)
	opts := docker.StatsOptions{
		ID:     container.ID,
		Stream: false,
		Stats:  statsC,
	}
	go func() {
		err := client.Stats(opts)
		if err != nil {
			log.Printf("[ERROR] cannot get stats for container %q: %s", container, err)
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

func sendMetrics(container *docker.Container, metrics map[string]string) error {
	st := getStatter(container)
	for key, value := range metrics {
		err := st.Send(key, value)
		if err != nil {
			log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			return err
		}
	}
	return nil
}

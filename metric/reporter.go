// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/container"
)

type Reporter struct {
	backend    statter
	infoClient *container.InfoClient
}

func (r *Reporter) Do() {
	containers, err := r.listContainers()
	if err != nil {
		log.Printf("[ERROR] failed to list containers: %s", err)
	}
	r.getMetrics(containers)
}

func (r *Reporter) listContainers() ([]docker.APIContainers, error) {
	return r.infoClient.GetClient().ListContainers(docker.ListContainersOptions{})
}

func (r *Reporter) getMetrics(containers []docker.APIContainers) {
	var wg sync.WaitGroup
	for _, container := range containers {
		wg.Add(1)
		go func(contID string) {
			defer wg.Done()
			container, err := r.infoClient.GetContainer(contID)
			if err != nil {
				log.Printf("[ERROR] cannot inspect container %q: %s", contID, err)
				return
			}
			stats, err := container.Stats()
			if err != nil {
				log.Printf("[ERROR] cannot get stats for container %q: %s", container, err)
				return
			}
			metrics, err := statsToMetricsMap(stats)
			if err != nil {
				log.Printf("[ERROR] failed to get metrics for container %q: %s", container, err)
				return
			}
			err = r.sendMetrics(container, metrics)
			if err != nil {
				log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			}
		}(container.ID)
	}
	wg.Wait()
}

func (r *Reporter) sendMetrics(container *container.Container, metrics map[string]string) error {
	for key, value := range metrics {
		err := r.backend.Send(container.AppName, container.Config.Hostname, container.ProcessName, key, value)
		if err != nil {
			log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			return err
		}
	}
	return nil
}

func (r *Reporter) sendConnMetrics(container *container.Container, conns []conn) error {
	for _, conn := range conns {
		var value string
		switch container.NetworkSettings.IPAddress {
		case conn.SourceIP:
			value = conn.DestinationIP + ":" + conn.DestinationPort
		case conn.DestinationIP:
			value = conn.SourceIP + ":" + conn.SourcePort
		}
		if value != "" {
			err := r.backend.Send(container.AppName, container.Config.Hostname, container.ProcessName, "connection", value)
			if err != nil {
				log.Printf("[ERROR] failed to send connection metrics for container %q: %s", container, err)
				return err
			}
		}
	}
	return nil
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
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

func getMetrics(dockerEndpoint string, containers []docker.APIContainers) {
	var wg sync.WaitGroup
	for _, container := range containers {
		wg.Add(1)
		go func(c docker.APIContainers) {
			defer wg.Done()
			container, err := getContainer(dockerEndpoint, c.ID)
			if err != nil {
				log.Printf("[ERROR] cannot inspect container %q dockerclient instance: %s", container, err)
				return
			}
			if !container.metricEnabled() {
				log.Printf("[INFO] metrics not enabled for container %q. Skipping.", container.ID)
				return
			}
			metrics, err := container.metrics(dockerEndpoint)
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

func sendMetrics(container *container, metrics map[string]string) error {
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

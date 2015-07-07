// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

var DockerEndpoint string

type statter interface {
	Send(key, value string) error
}

func getStatter(container *docker.Container) statter {
	statters := map[string]statter{
		"statsd":   &statsd{},
		"logstash": &logStash{},
	}
	if container.Config != nil {
		for _, val := range container.Config.Env {
			if strings.HasPrefix(val, "TSURU_METRICS_BACKEND") {
				statterName := strings.Replace(val, "TSURU_METRICS_BACKEND=", "", -1)
				st, ok := statters[statterName]
				if ok {
					return st
				}
			}
		}
	}
	return &fake{}
}

func reportMetrics() {
	client, err := docker.NewClient(DockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return
	}
	containers, err := client.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		log.Printf("[ERROR] failed to list containers in the Docker server at %q: %s", DockerEndpoint, err)
		return
	}
	getMetrics(containers)
}

func metricsEnabled(container *docker.Container) bool {
	for _, val := range container.Config.Env {
		if strings.HasPrefix(val, "TSURU_METRICS_BACKEND") {
			return true
		}
	}
	return false
}

func getMetrics(containers []docker.APIContainers) {
	var wg sync.WaitGroup
	for _, container := range containers {
		wg.Add(1)
		go func(c docker.APIContainers) {
			defer wg.Done()
			client, err := docker.NewClient(DockerEndpoint)
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
			metrics, err := getMetricFromContainer(container)
			if err != nil {
				log.Printf("[ERROR] failed to get metrics for container %q in the Docker server at %q: %s", container, DockerEndpoint, err)
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

func getMetricFromContainer(container *docker.Container) (map[string]string, error) {
	return nil, nil
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

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"reflect"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

type Reporter struct {
	DockerEndpoint string
	Backend        string
}

func (r *Reporter) Do() {
	st := r.statter()
	// don't run reporter when the statter is the fake statter
	if reflect.ValueOf(st).Type().AssignableTo(reflect.ValueOf(fake{}).Type()) {
		return
	}
	containers, err := r.listContainers()
	if err != nil {
		log.Printf("[ERROR] failed to list containers in the Docker server at %q: %s", r.DockerEndpoint, err)
	}
	r.getMetrics(containers)
}

func (r *Reporter) listContainers() ([]docker.APIContainers, error) {
	client, err := docker.NewClient(r.DockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return nil, err
	}
	return client.ListContainers(docker.ListContainersOptions{})
}

func (r *Reporter) getMetrics(containers []docker.APIContainers) {
	var wg sync.WaitGroup
	for _, container := range containers {
		wg.Add(1)
		go func(c docker.APIContainers) {
			defer wg.Done()
			container, err := getContainer(r.DockerEndpoint, c.ID)
			if err != nil {
				log.Printf("[ERROR] cannot inspect container %q dockerclient instance: %s", container, err)
				return
			}
			metrics, err := container.metrics(r.DockerEndpoint)
			if err != nil {
				log.Printf("[ERROR] failed to get metrics for container %q in the Docker server at %q: %s", container, r.DockerEndpoint, err)
				return
			}
			err = r.sendMetrics(container, metrics)
			if err != nil {
				log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			}
		}(container)
	}
	wg.Wait()
}

func (r *Reporter) sendMetrics(container *container, metrics map[string]string) error {
	appName := container.appName()
	process := container.process()
	for key, value := range metrics {
		err := r.statter().Send(appName, container.Config.Hostname, process, key, value)
		if err != nil {
			log.Printf("[ERROR] failed to send metrics for container %q: %s", container, err)
			return err
		}
	}
	return nil
}

func (r *Reporter) statter() statter {
	statters := map[string]statter{
		"statsd":   newStatsd(),
		"logstash": newLogStash(),
	}
	st, ok := statters[r.Backend]
	if ok {
		return st
	}
	return &fake{}
}

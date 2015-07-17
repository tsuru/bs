// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"reflect"
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/container"
)

type Reporter struct {
	backend    string
	infoClient *container.InfoClient
}

func (r *Reporter) Do() {
	st := r.statter()
	// don't run reporter when the statter is the fake statter
	if reflect.ValueOf(st).Type().AssignableTo(reflect.ValueOf(fake{}).Type()) {
		return
	}
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
	conns, err := conntrack()
	if err != nil {
		log.Printf("[ERROR] failed to run collect conntrack information: %s", err)
	}
	for _, container := range containers {
		wg.Add(1)
		go func(c docker.APIContainers) {
			defer wg.Done()
			container, err := r.infoClient.GetContainer(c.ID)
			if err != nil {
				log.Printf("[ERROR] cannot inspect container %q: %s", c.ID, err)
				return
			}
			metrics, err := statsToMetricsMap(container, conns)
			if err != nil {
				log.Printf("[ERROR] failed to get metrics for container %q: %s", container, err)
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

func (r *Reporter) sendMetrics(container *container.Container, metrics map[string]string) error {
	for key, value := range metrics {
		err := r.statter().Send(container.AppName, container.Config.Hostname, container.ProcessName, key, value)
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
	st, ok := statters[r.backend]
	if ok {
		return st
	}
	return &fake{}
}

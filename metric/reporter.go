// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"sync"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/container"
)

type Reporter struct {
	backend               statter
	infoClient            *container.InfoClient
	containerSelectionEnv string
}

func (r *Reporter) Do() {
	containers, err := r.infoClient.ListContainers()
	if err != nil {
		bslog.Errorf("failed to list containers: %s", err)
	}
	var selectionEnvs []string
	if r.containerSelectionEnv != "" {
		selectionEnvs = []string{r.containerSelectionEnv}
	}
	r.getMetrics(containers, selectionEnvs)
	err = r.getHostMetrics()
	if err != nil {
		bslog.Errorf("failed to get host metrics: %s", err)
	}
}

func (r *Reporter) getMetrics(containers []docker.APIContainers, selectionEnvs []string) {
	var wg sync.WaitGroup
	conns, err := conntrack()
	if err != nil {
		bslog.Errorf("failed to execute conntrack: %s", err)
	}
	for _, cont := range containers {
		wg.Add(1)
		go func(contID string) {
			defer wg.Done()
			cont, err := r.infoClient.GetContainer(contID, true, selectionEnvs)
			if err != nil {
				if err != container.ErrTsuruVariablesNotFound {
					bslog.Errorf("cannot inspect container %q: %s", contID, err)
				}
				return
			}
			stats, err := cont.Stats()
			if err != nil || stats == nil {
				bslog.Errorf("cannot get stats for container %v: %s", cont, err)
				return
			}
			metrics, err := statsToMetricsMap(stats)
			if err != nil {
				bslog.Errorf("failed to get metrics for container %v: %s", cont, err)
				return
			}
			err = r.sendMetrics(cont, metrics)
			if err != nil {
				bslog.Errorf("failed to send metrics for container %v: %s", cont, err)
			}
			err = r.sendConnMetrics(cont, conns)
			if err != nil {
				bslog.Errorf("failed to send conn metrics for container %v: %s", cont, err)
			}
		}(cont.ID)
	}
	wg.Wait()
}

func (r *Reporter) sendMetrics(container *container.Container, metrics map[string]float) error {
	for key, value := range metrics {
		err := r.backend.Send(NewContainerInfo(container), key, value)
		if err != nil {
			bslog.Errorf("failed to send metrics for container %q: %s", container, err)
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
			err := r.backend.SendConn(NewContainerInfo(container), value)
			if err != nil {
				bslog.Errorf("failed to send connection metrics for container %q: %s", container, err)
				return err
			}
		}
	}
	return nil
}

func (r *Reporter) getHostMetrics() error {
	hostClient, err := NewHostClient()
	if err != nil {
		return err
	}
	metrics, err := hostClient.GetHostMetrics()
	if err != nil {
		return err
	}
	hostname, err := hostClient.GetHostname()
	if err != nil {
		return err
	}
	for _, metric := range metrics {
		err := r.sendHostMetrics(hostname, metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reporter) sendHostMetrics(hostname string, metrics map[string]float) error {
	for key, value := range metrics {
		err := r.backend.SendHost(hostname, key, value)
		if err != nil {
			bslog.Errorf("failed to send host metric %s: %s", key, err)
			return err
		}
	}
	return nil
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
)

type container struct {
	ID     string
	Name   string
	Status string
}

type respUnit struct {
	ID    string
	Found bool
}

type ReporterConfig struct {
	Interval       time.Duration
	DockerEndpoint string
	TsuruEndpoint  string
	TsuruToken     string
	AppNameEnvVar  string
}

type Reporter struct {
	config *ReporterConfig
	abort  chan<- struct{}
	exit   <-chan struct{}
}

// NewReporter starts the status reporter. It will run intermitently, sending a
// message in the exit channel in case it exits. It's possible to arbitrarily
// interrupt the reporter by sending a message in the abort channel.
func NewReporter(config *ReporterConfig) *Reporter {
	abort := make(chan struct{})
	exit := make(chan struct{})
	reporter := Reporter{
		config: config,
		abort:  abort,
		exit:   exit,
	}
	go func(abort <-chan struct{}) {
		for {
			select {
			case <-abort:
				close(exit)
				return
			case <-time.After(reporter.config.Interval):
				reporter.reportStatus()
			}
		}
	}(abort)
	return &reporter
}

// Stop stops the reporter. It will block until it actually stops (i.e. there's
// no need to call Wait after calling Stop).
func (r *Reporter) Stop() {
	close(r.abort)
	<-r.exit
}

// Wait blocks until the reporter stops.
func (r *Reporter) Wait() {
	<-r.exit
}

func (r *Reporter) reportStatus() {
	client, err := docker.NewClient(r.config.DockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return
	}
	opts := docker.ListContainersOptions{All: true}
	containers, err := client.ListContainers(opts)
	if err != nil {
		log.Printf("[ERROR] failed to list containers in the Docker server at %q: %s", r.config.DockerEndpoint, err)
		return
	}
	resp, err := r.updateUnits(containers)
	if err != nil {
		log.Printf("[ERROR] failed to send data to the tsuru server at %q: %s", r.config.TsuruEndpoint, err)
		return
	}
	err = r.handleTsuruResponse(resp)
	if err != nil {
		log.Printf("[ERROR] failed to handle tsuru response: %s", err)
	}
}

func (r *Reporter) updateUnits(containers []docker.APIContainers) ([]respUnit, error) {
	payload := make([]container, len(containers))
	client, err := docker.NewClient(r.config.DockerEndpoint)
	if err != nil {
		return nil, err
	}
	for i, c := range containers {
		var status string
		cont, err := client.InspectContainer(c.ID)
		if err != nil {
			log.Printf("[ERROR] failed to inspect container %q: %s", c.ID, err)
			status = "error"
		} else {
			if cont.State.Restarting {
				status = "error"
			} else if cont.State.Running {
				status = "started"
			} else {
				status = "stopped"
			}
		}
		var name string
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		payload[i] = container{ID: c.ID, Name: name, Status: status}
	}
	var body bytes.Buffer
	err = json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/units/status", strings.TrimRight(r.config.TsuruEndpoint, "/"))
	request, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "bearer "+r.config.TsuruToken)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	var statusResp []respUnit
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&statusResp)
	if err != nil {
		return nil, err
	}
	return statusResp, nil
}

func (r *Reporter) handleTsuruResponse(resp []respUnit) error {
	goneUnits := make([]string, 0, len(resp))
	for _, unit := range resp {
		if !unit.Found {
			goneUnits = append(goneUnits, unit.ID)
		}
	}
	client, err := docker.NewClient(r.config.DockerEndpoint)
	if err != nil {
		return err
	}
	for _, id := range goneUnits {
		container, err := client.InspectContainer(id)
		if err != nil {
			log.Printf("[ERROR] failed to inspect container %q: %s", id, err)
			continue
		}
		for _, env := range container.Config.Env {
			if strings.HasPrefix(env, r.config.AppNameEnvVar) {
				opts := docker.RemoveContainerOptions{ID: id, Force: true}
				err = client.RemoveContainer(opts)
				if err != nil {
					log.Printf("[ERROR] failed to remove container %q: %s", id, err)
				}
				break
			}
		}
	}
	return nil
}

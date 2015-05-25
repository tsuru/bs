// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/fsouza/go-dockerclient"
)

type container struct {
	ID     string
	Status string
}

type respUnit struct {
	ID    string
	Found bool
}

func collectStatus() {
	client, err := docker.NewClient(config.DockerEndpoint)
	if err != nil {
		log.Printf("[ERROR] cannot create dockerclient instance: %s", err)
		return
	}
	opts := docker.ListContainersOptions{All: true}
	containers, err := client.ListContainers(opts)
	if err != nil {
		log.Printf("[ERROR] failed to list containers in the Docker server at %q: %s", config.DockerEndpoint, err)
		return
	}
	resp, err := updateUnits(containers)
	if err != nil {
		log.Printf("[ERROR] failed to send data to the tsuru server at %q: %s", config.TsuruEndpoint, err)
		return
	}
	err = handleTsuruResponse(resp)
	if err != nil {
		log.Printf("[ERROR] failed to handle tsuru response: %s", err)
	}
}

func updateUnits(containers []docker.APIContainers) ([]respUnit, error) {
	payload := make([]container, len(containers))
	client, err := docker.NewClient(config.DockerEndpoint)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	for i, c := range containers {
		wg.Add(1)
		go func(i int, c docker.APIContainers) {
			defer wg.Done()
			var status string
			cont, err := client.InspectContainer(c.ID)
			if err != nil {
				log.Printf("[ERROR] failed to instpect container %q: %s", c.ID, err)
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
			payload[i] = container{ID: c.ID, Status: status}
		}(i, c)
	}
	wg.Wait()
	var body bytes.Buffer
	err = json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/units/status", strings.TrimRight(config.TsuruEndpoint, "/"))
	request, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "bearer "+config.TsuruToken)
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

func handleTsuruResponse(resp []respUnit) error {
	goneUnits := make([]string, 0, len(resp))
	for _, unit := range resp {
		if !unit.Found {
			goneUnits = append(goneUnits, unit.ID)
		}
	}
	client, err := docker.NewClient(config.DockerEndpoint)
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
			if strings.HasPrefix(env, config.SentinelEnvVar) {
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

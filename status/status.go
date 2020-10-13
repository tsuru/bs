// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ajg/form"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/container"
	node "github.com/tsuru/bs/node"
	"github.com/tsuru/tsuru/provision"
)

type containerStatus struct {
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
	Kubernetes     bool
}

type Reporter struct {
	config     *ReporterConfig
	abort      chan<- struct{}
	exit       <-chan struct{}
	checks     *checkCollection
	addrs      []string
	infoClient *container.InfoClient
	httpClient *http.Client
	mu         sync.Mutex
	removeMap  map[string]chan struct{}
}

type hostStatus struct {
	Addrs  []string
	Units  []containerStatus
	Checks []hostCheckResult
}

const (
	dialTimeout = 10 * time.Second
	fullTimeout = 1 * time.Minute
)

var errRouteNotFound = errors.New("route not found")

// NewReporter starts the status reporter. It will run intermitently, sending a
// message in the exit channel in case it exits. It's possible to arbitrarily
// interrupt the reporter by sending a message in the abort channel.
func NewReporter(config *ReporterConfig) (*Reporter, error) {
	if config.TsuruEndpoint == "" {
		return nil, errors.New("tsuru endpoint must be set for status reporting")
	}
	abort := make(chan struct{})
	exit := make(chan struct{})
	infoClient, err := container.NewClient(config.DockerEndpoint)
	if err != nil {
		return nil, err
	}
	checks := NewCheckCollection(infoClient.GetClient())
	addrs, err := node.GetNodeAddrs()
	if err != nil {
		return nil, fmt.Errorf("[status reporter] unable to get network addresses: %s", err)
	}
	transport := http.Transport{
		Dial: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: dialTimeout,
		IdleConnTimeout:     30 * time.Second,
	}
	reporter := Reporter{
		config:     config,
		abort:      abort,
		exit:       exit,
		infoClient: infoClient,
		checks:     checks,
		addrs:      addrs,
		httpClient: &http.Client{
			Transport: &transport,
			Timeout:   fullTimeout,
		},
		removeMap: make(map[string]chan struct{}),
	}
	go func(abort <-chan struct{}) {
		for {
			reporter.reportStatus()
			select {
			case <-abort:
				close(exit)
				return
			case <-time.After(reporter.config.Interval):
			}
		}
	}(abort)
	return &reporter, nil
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
	hostChecks := r.checks.Run()
	hostData := &hostStatus{
		Addrs:  r.addrs,
		Checks: hostChecks,
	}

	if !r.config.Kubernetes {
		client := r.infoClient.GetClient()
		opts := docker.ListContainersOptions{All: true}
		containers, err := client.ListContainers(opts)
		if err != nil {
			bslog.Errorf("[status reporter] failed to list containers in the Docker server at %q: %s", r.config.DockerEndpoint, err)
			return
		}
		hostData.Units = r.retrieveContainerStatuses(containers)
	}

	resp, err := r.updateNode(hostData)
	if err == errRouteNotFound {
		resp, err = r.updateUnits(hostData.Units)
	}
	if err != nil {
		bslog.Errorf("[status reporter] failed to send data to the tsuru server at %q: %s", r.config.TsuruEndpoint, err)
		return
	}
	err = r.handleTsuruResponse(resp)
	if err != nil {
		bslog.Errorf("[status reporter] failed to handle tsuru response: %s", err)
	}
}

func (r *Reporter) retrieveContainerStatuses(containers []docker.APIContainers) []containerStatus {
	statuses := make([]containerStatus, 0, len(containers))
	for _, c := range containers {
		var name string
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		var status provision.Status
		cont, err := r.infoClient.GetAppContainer(c.ID, false)
		if err == container.ErrTsuruVariablesNotFound {
			continue
		}
		if err != nil {
			bslog.Errorf("[status reporter] failed to inspect container %q (%s): %s", c.ID, name, err)
			continue
		}
		if cont != nil && cont.IsIsolated() {
			continue
		}
		if cont.Container.State.Restarting ||
			cont.Container.State.Dead ||
			cont.Container.State.RemovalInProgress {
			status = provision.StatusError
		} else if cont.Container.State.Running {
			status = provision.StatusStarted
		} else if cont.Container.State.StartedAt.IsZero() {
			status = provision.StatusCreated
		} else {
			status = provision.StatusStopped
		}
		statuses = append(statuses, containerStatus{ID: c.ID, Name: name, Status: status.String()})
	}
	return statuses
}

func (r *Reporter) updateNode(payload *hostStatus) (*http.Response, error) {
	bodyContent, err := form.EncodeToString(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/node/status", strings.TrimRight(r.config.TsuruEndpoint, "/"))
	request, err := http.NewRequest("POST", url, strings.NewReader(bodyContent))
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Add("Authorization", "bearer "+r.config.TsuruToken)
	resp, err := r.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errRouteNotFound
	}
	return resp, nil
}

func (r *Reporter) updateUnits(payload []containerStatus) (*http.Response, error) {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(payload)
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
	resp, err := r.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (r *Reporter) tryRemoveContainer(id string) {
	r.mu.Lock()
	if _, inSet := r.removeMap[id]; inSet {
		r.mu.Unlock()
		return
	}
	ch := make(chan struct{})
	r.removeMap[id] = ch
	r.mu.Unlock()
	go func() {
		defer func() {
			close(ch)
			r.mu.Lock()
			delete(r.removeMap, id)
			r.mu.Unlock()
		}()
		bslog.Warnf("[status reporter] removing container %q not found in tsuru response", id)
		client := r.infoClient.GetClient()
		opts := docker.RemoveContainerOptions{ID: id, Force: true}
		err := client.RemoveContainer(opts)
		if err != nil {
			bslog.Errorf("[status reporter] failed to remove invalid container %q: %s", id, err)
		}
	}()
}

func (r *Reporter) waitPendingRemovals() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ch := range r.removeMap {
		<-ch
	}
}

func (r *Reporter) handleTsuruResponse(resp *http.Response) error {
	var statusResp []respUnit
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from tsuru %d: %s", resp.StatusCode, string(body))
	}
	if r.config.Kubernetes {
		return nil
	}
	err := json.NewDecoder(resp.Body).Decode(&statusResp)
	if err != nil {
		return fmt.Errorf("unable to parse tsuru response: %s", err)
	}
	for _, unit := range statusResp {
		if !unit.Found {
			r.tryRemoveContainer(unit.ID)
		}
	}
	return nil
}

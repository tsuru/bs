// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"fmt"
	"os"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/container"
)

type runner struct {
	dockerClientInfo   *config.DockerConfig
	interval           time.Duration
	metricsBackend     string
	abort              chan struct{}
	exit               chan struct{}
	EnableBasicMetrics bool
	EnableConnMetrics  bool
	EnableHostMetrics  bool
}

func NewRunner(dockerClientInfo *config.DockerConfig, interval time.Duration, metricsBackend string) *runner {
	return &runner{
		abort:            make(chan struct{}),
		exit:             make(chan struct{}),
		dockerClientInfo: dockerClientInfo,
		interval:         interval,
		metricsBackend:   metricsBackend,
	}
}

// Start starts a reporter that will send metrics to a backend until there is
// a message in the exit channel. It's possible to interrupt the runner by
// sending a message in the abort channel.
func (r *runner) Start() (err error) {
	defer func() {
		if err != nil {
			close(r.exit)
		}
	}()
	client, err := container.NewClient(r.dockerClientInfo)
	if err != nil {
		return
	}
	containerSelectionEnv := os.Getenv("CONTAINER_SELECTION_ENV")
	constructor := backends[r.metricsBackend]
	if constructor == nil {
		err = fmt.Errorf("no metrics backend found with name %q", r.metricsBackend)
		return
	}
	backend, err := constructor()
	if err != nil {
		return
	}
	hostClient, err := NewHostClient()
	if err != nil {
		bslog.Warnf("Failed to create host client: %s", err)
		err = nil
	}
	reporter := &Reporter{
		backend:               backend,
		infoClient:            client,
		containerSelectionEnv: containerSelectionEnv,
		hostClient:            hostClient,
		enableBasicMetrics:    r.EnableBasicMetrics,
		enableConnMetrics:     r.EnableConnMetrics,
		enableHostMetrics:     r.EnableHostMetrics,
	}
	go func() {
		for {
			reporter.Do()
			select {
			case <-r.abort:
				close(r.exit)
				return
			case <-time.After(r.interval):
			}

		}
	}()
	return
}

// Stop stops the runner.
func (r *runner) Stop() {
	close(r.abort)
	<-r.exit
}

// Wait blocks until the runner stops.
func (r *runner) Wait() {
	<-r.exit
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"fmt"
	"os"
	"time"

	"github.com/tsuru/bs/container"
)

type runner struct {
	dockerEndpoint string
	interval       time.Duration
	abort          chan struct{}
	exit           chan struct{}
}

var statters = map[string]func() (statter, error){
	"logstash": newLogStash,
}

func NewRunner(dockerEndpoint string, interval time.Duration) *runner {
	return &runner{
		abort:          make(chan struct{}),
		exit:           make(chan struct{}),
		dockerEndpoint: dockerEndpoint,
		interval:       interval,
	}
}

// Start starts a reporter that will send metrics to a backend until there is
// a message in the exit channel. It's possible to interrupt the runner by
// sending a message in the abort channel.
func (r *runner) Start() error {
	client, err := container.NewClient(r.dockerEndpoint)
	if err != nil {
		return err
	}
	backendName := os.Getenv("METRICS_BACKEND")
	constructor := statters[backendName]
	if constructor == nil {
		return fmt.Errorf("no metrics backend found with name %q", backendName)
	}
	backend, err := constructor()
	if err != nil {
		return err
	}
	reporter := &Reporter{
		backend:    backend,
		infoClient: client,
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
	return nil
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

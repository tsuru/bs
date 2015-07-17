// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"os"
	"time"

	"github.com/tsuru/bs/container"
)

type Runner struct {
	DockerEndpoint string
	Interval       time.Duration
	finish         chan bool
}

func (r *Runner) Start() error {
	client, err := container.NewClient(r.DockerEndpoint)
	if err != nil {
		return err
	}
	reporter := &Reporter{
		backend:    os.Getenv("METRICS_BACKEND"),
		infoClient: client,
	}
	go func() {
		for {
			reporter.Do()
			select {
			case <-r.finish:
				return
			case <-time.After(r.Interval):
			}

		}
	}()
	return nil
}

func (r *Runner) Stop() {
	r.finish <- true
}

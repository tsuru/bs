// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"os"
	"time"
)

type Runner struct {
	DockerEndpoint string
	Interval       time.Duration
	finish         chan bool
}

func (r *Runner) Start() {
	go func() {
		for {
			reporter := &Reporter{
				DockerEndpoint: r.DockerEndpoint,
				Backend:        os.Getenv("METRICS_BACKEND"),
			}
			reporter.Do()
			select {
			case <-r.finish:
				return
			case <-time.After(r.Interval):
			}
		}
	}()
}

func (r *Runner) Stop() {
	r.finish <- true
}

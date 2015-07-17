// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/container"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (s *S) TestSendMetrics(c *check.C) {
	var cont container.Container
	config := docker.Config{}
	cont.Config = &config
	r := &Reporter{}
	err := r.sendMetrics(&cont, nil)
	c.Assert(err, check.IsNil)
}

func (s *S) TestGetMetrics(c *check.C) {
	var containers []docker.APIContainers
	r := &Reporter{}
	r.getMetrics(containers)
}

func (s *S) TestReporterStatter(c *check.C) {
	backends := map[string]statter{
		"fake":     &fake{},
		"logstash": &logStash{},
		"statsd":   &statsd{},
	}
	for b, st := range backends {
		r := &Reporter{backend: b}
		c.Check(r.statter(), check.FitsTypeOf, st)
	}
}

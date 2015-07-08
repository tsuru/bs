// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

func (S) TestGetStatter(c *check.C) {
	var cont container
	config := docker.Config{}
	cont.Config = &config
	st := getStatter(&cont)
	var s statter
	c.Assert(st, check.Implements, &s)
	c.Assert(st, check.FitsTypeOf, &fake{})
	config = docker.Config{
		Env: []string{"TSURU_METRICS_BACKEND=logstash"},
	}
	cont.Config = &config
	st = getStatter(&cont)
	c.Assert(st, check.Implements, &s)
	c.Assert(st, check.FitsTypeOf, &logStash{})
	config = docker.Config{
		Env: []string{"TSURU_METRICS_BACKEND=statsd"},
	}
	cont.Config = &config
	st = getStatter(&cont)
	c.Assert(st, check.Implements, &s)
	c.Assert(st, check.FitsTypeOf, &statsd{})
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"testing"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (S) TestSendMetrics(c *check.C) {
	var cont container
	err := sendMetrics(&cont, nil)
	c.Assert(err, check.IsNil)
}

func (S) TestGetMetrics(c *check.C) {
	var containers []docker.APIContainers
	getMetrics("", containers)
}

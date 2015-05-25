// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"testing"

	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (S) TestLoadConfig(c *check.C) {
	os.Setenv("DOCKER_ENDPOINT", "http://192.168.50.4:2375")
	os.Setenv("TSURU_ENDPOINT", "http://192.168.50.4:8080")
	os.Setenv("TSURU_SENTINEL_ENV_VAR", "TSURU_APP_NAME")
	os.Setenv("TSURU_TOKEN", "sometoken")
	loadConfig()
	c.Check(config.DockerEndpoint, check.Equals, "http://192.168.50.4:2375")
	c.Check(config.TsuruEndpoint, check.Equals, "http://192.168.50.4:8080")
	c.Check(config.TsuruToken, check.Equals, "sometoken")
	c.Check(config.SentinelEnvVar, check.Equals, "TSURU_APP_NAME=")
}

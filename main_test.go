// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

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
	os.Setenv("STATUS_INTERVAL", "45")
	os.Setenv("SYSLOG_LISTEN_ADDRESS", "udp://0.0.0.0:1514")
	os.Setenv("SYSLOG_FORWARD_ADDRESSES", "udp://10.0.0.1:514,tcp://10.0.0.2:514")
	loadConfig()
	c.Check(config.DockerEndpoint, check.Equals, "http://192.168.50.4:2375")
	c.Check(config.TsuruEndpoint, check.Equals, "http://192.168.50.4:8080")
	c.Check(config.TsuruToken, check.Equals, "sometoken")
	c.Check(config.SentinelEnvVar, check.Equals, "TSURU_APP_NAME=")
	c.Check(config.StatusInterval, check.Equals, time.Duration(45e9))
	c.Check(config.SyslogListenAddress, check.Equals, "udp://0.0.0.0:1514")
	c.Check(config.SyslogForwardAddresses, check.DeepEquals, []string{
		"udp://10.0.0.1:514",
		"tcp://10.0.0.2:514",
	})
}

func (S) TestLoadConfigInvalidDuration(c *check.C) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	os.Setenv("DOCKER_ENDPOINT", "http://192.168.50.4:2375")
	os.Setenv("TSURU_ENDPOINT", "http://192.168.50.4:8080")
	os.Setenv("TSURU_SENTINEL_ENV_VAR", "TSURU_APP_NAME")
	os.Setenv("TSURU_TOKEN", "sometoken")
	os.Setenv("STATUS_INTERVAL", "four")
	loadConfig()
	c.Check(config.DockerEndpoint, check.Equals, "http://192.168.50.4:2375")
	c.Check(config.TsuruEndpoint, check.Equals, "http://192.168.50.4:8080")
	c.Check(config.TsuruToken, check.Equals, "sometoken")
	c.Check(config.SentinelEnvVar, check.Equals, "TSURU_APP_NAME=")
	c.Check(config.StatusInterval, check.Equals, time.Duration(60e9))
	c.Assert(buf.String(), check.Matches, `(?m).*\[WARNING\] invalid interval "four"\. Using the default value of 60 seconds$`)
}

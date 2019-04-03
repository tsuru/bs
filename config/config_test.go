// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"bytes"
	"log"
	"os"
	"testing"
	"time"

	"github.com/tsuru/bs/bslog"
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
	os.Setenv("TSURU_TOKEN", "sometoken")
	os.Setenv("STATUS_INTERVAL", "45")
	os.Setenv("SYSLOG_LISTEN_ADDRESS", "udp://0.0.0.0:1514")
	os.Setenv("LOG_BACKENDS", "b1, b2 ")
	LoadConfig()
	c.Check(Config.DockerEndpoint, check.Equals, "http://192.168.50.4:2375")
	c.Check(Config.TsuruEndpoint, check.Equals, "http://192.168.50.4:8080")
	c.Check(Config.TsuruToken, check.Equals, "sometoken")
	c.Check(Config.StatusInterval, check.Equals, time.Duration(45e9))
	c.Check(Config.SyslogListenAddress, check.Equals, "udp://0.0.0.0:1514")
	c.Check(Config.LogBackends, check.DeepEquals, []string{"b1", "b2"})
}

func (S) TestLoadConfigInvalidDuration(c *check.C) {
	var buf bytes.Buffer
	bslog.Logger = log.New(&buf, "", 0)
	defer func() { bslog.Logger = log.New(os.Stderr, "", log.LstdFlags) }()
	os.Setenv("DOCKER_ENDPOINT", "http://192.168.50.4:2375")
	os.Setenv("TSURU_ENDPOINT", "http://192.168.50.4:8080")
	os.Setenv("TSURU_TOKEN", "sometoken")
	os.Setenv("STATUS_INTERVAL", "four")
	os.Setenv("HOST_PROC", "/prochost")
	LoadConfig()
	c.Check(Config.DockerEndpoint, check.Equals, "http://192.168.50.4:2375")
	c.Check(Config.TsuruEndpoint, check.Equals, "http://192.168.50.4:8080")
	c.Check(Config.TsuruToken, check.Equals, "sometoken")
	c.Check(Config.StatusInterval, check.Equals, time.Duration(60e9))
	c.Assert(buf.String(), check.Matches, `(?m).*\[WARNING\] invalid value for STATUS_INTERVAL\. Using the default value of 60$`)
}

func (S) TestLoadConfigDefaultLogBackends(c *check.C) {
	os.Unsetenv("LOG_BACKENDS")
	LoadConfig()
	c.Check(Config.LogBackends, check.DeepEquals, []string{"tsuru", "syslog"})
}

func (S) TestBoolEnvOrDefault(c *check.C) {
	v := BoolEnvOrDefault(true, "BOOL_ENV")
	c.Assert(v, check.Equals, true)
	v = BoolEnvOrDefault(false, "BOOL_ENV")
	c.Assert(v, check.Equals, false)
	os.Setenv("BOOL_ENV", "1")
	v = BoolEnvOrDefault(false, "BOOL_ENV")
	c.Assert(v, check.Equals, true)
	os.Setenv("BOOL_ENV", "0")
	v = BoolEnvOrDefault(true, "BOOL_ENV")
	c.Assert(v, check.Equals, false)
}

func (S) TestStringsEnvOrDefault(c *check.C) {
	var buf bytes.Buffer
	bslog.Logger = log.New(&buf, "", 0)
	defer func() { bslog.Logger = log.New(os.Stderr, "", log.LstdFlags) }()
	v := StringsEnvOrDefault(nil, "STRINGS_ENV")
	c.Assert(v, check.IsNil)
	c.Assert(buf.String(), check.Equals, "")
	v = StringsEnvOrDefault([]string{"a"}, "STRINGS_ENV")
	c.Assert(v, check.DeepEquals, []string{"a"})
	c.Assert(buf.String(), check.Matches, `(?m).*\[WARNING\] invalid value for STRINGS_ENV\. Using the default value of \[a\]$`)
	buf.Reset()
	os.Setenv("STRINGS_ENV", "myvalue")
	v = StringsEnvOrDefault(nil, "STRINGS_ENV")
	c.Assert(v, check.DeepEquals, []string{"myvalue"})
	c.Assert(buf.String(), check.Equals, "")
	os.Setenv("STRINGS_ENV", "myvalue , other,value, ok ")
	v = StringsEnvOrDefault(nil, "STRINGS_ENV")
	c.Assert(v, check.DeepEquals, []string{"myvalue", "other", "value", "ok"})
	c.Assert(buf.String(), check.Equals, "")
}

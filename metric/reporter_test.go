// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"errors"
	"sort"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/container"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (s *S) SetUpTest(c *check.C) {
	fakeBackend.reset()
}

func (s *S) createContainer() container.Container {
	return container.Container{
		Container: docker.Container{
			Config:          &docker.Config{},
			NetworkSettings: &docker.NetworkSettings{IPAddress: "172.17.0.27"},
		},
		ShortHostname: "afdb3737ff",
		AppName:       "myapp",
		ProcessName:   "myprocess",
		TsuruApp:      true,
	}
}

func (s *S) TestSendMetrics(c *check.C) {
	cont := s.createContainer()
	r := Reporter{backend: &fakeBackend}
	metrics := map[string]float{"cpu": float(900), "mem": float(512)}
	err := r.sendMetrics(&cont, metrics)
	c.Assert(err, check.IsNil)
	expected := []fakeStat{
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "cpu", value: float(900)},
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "mem", value: float(512)},
	}
	if fakeBackend.stats[0].key != "cpu" {
		expected[0], expected[1] = expected[1], expected[0]
	}
	c.Assert(fakeBackend.stats, check.DeepEquals, expected)
}

func (s *S) TestSendMetricsFailure(c *check.C) {
	cont := s.createContainer()
	r := Reporter{backend: &fakeBackend}
	prepErr := errors.New("something went wrong")
	fakeBackend.prepareFailure(prepErr)
	err := r.sendMetrics(&cont, map[string]float{"cpu": float(256)})
	c.Assert(err, check.Equals, prepErr)
}

func (s *S) TestSendConnMetrics(c *check.C) {
	cont := s.createContainer()
	conns := []conn{
		{SourceIP: "192.168.50.4", SourcePort: "33404", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "172.17.42.1", SourcePort: "42418", DestinationIP: "172.17.0.27", DestinationPort: "4001"},
		{SourceIP: "172.17.42.1", SourcePort: "42428", DestinationIP: "172.17.0.27", DestinationPort: "4001"},
		{SourceIP: "192.168.50.4", SourcePort: "53922", DestinationIP: "192.168.50.4", DestinationPort: "5000"},
		{SourceIP: "192.168.50.4", SourcePort: "43227", DestinationIP: "192.168.50.4", DestinationPort: "8080"},
		{SourceIP: "172.17.0.27", SourcePort: "39502", DestinationIP: "172.17.42.1", DestinationPort: "4001"},
		{SourceIP: "192.168.50.4", SourcePort: "33496", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "192.168.50.4", SourcePort: "33495", DestinationIP: "192.168.50.4", DestinationPort: "2375"},
		{SourceIP: "10.211.55.2", SourcePort: "51388", DestinationIP: "10.211.55.184", DestinationPort: "22"},
		{SourceIP: "172.17.0.27", SourcePort: "39492", DestinationIP: "172.17.42.1", DestinationPort: "4001"},
		{SourceIP: "172.17.0.27", SourcePort: "39492", DestinationIP: "192.168.50.4", DestinationPort: "8080"},
		{SourceIP: "10.211.55.2", SourcePort: "51370", DestinationIP: "10.211.55.184", DestinationPort: "22"},
	}
	r := Reporter{backend: &fakeBackend}
	err := r.sendConnMetrics(&cont, conns)
	c.Assert(err, check.IsNil)
	expected := []fakeStat{
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "connection", value: "172.17.42.1:42418"},
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "connection", value: "172.17.42.1:42428"},
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "connection", value: "172.17.42.1:4001"},
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "connection", value: "172.17.42.1:4001"},
		{app: "myapp", hostname: "afdb3737ff", process: "myprocess", key: "connection", value: "192.168.50.4:8080"},
	}
	c.Assert(fakeBackend.stats, check.DeepEquals, expected)
}

func (s *S) TestSendConnMetricsFailure(c *check.C) {
	cont := s.createContainer()
	r := Reporter{backend: &fakeBackend}
	prepErr := errors.New("something went wrong")
	fakeBackend.prepareFailure(prepErr)
	conns := []conn{{SourceIP: "172.17.0.27"}}
	err := r.sendConnMetrics(&cont, conns)
	c.Assert(err, check.Equals, prepErr)
}

func (s *S) TestGetMetrics(c *check.C) {
	bogusContainers := s.buildContainers()
	dockerServer, conts := s.startDockerServer(bogusContainers, nil, c)
	s.prepareStats(dockerServer, conts)
	client, err := container.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	defer dockerServer.Stop()
	r := Reporter{backend: &fakeBackend, infoClient: client}
	containers := make([]docker.APIContainers, len(conts))
	containers[0] = docker.APIContainers{ID: conts[0].ID}
	containers[1] = docker.APIContainers{ID: conts[1].ID, State: "restarting"}
	containers[2] = docker.APIContainers{ID: conts[2].ID, State: "running"}
	r.enableBasicMetrics = true
	r.getMetrics(containers, []string{})
	id0 := conts[0].ID[:12]
	id1 := conts[2].ID[:12]
	expected := []fakeStat{
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "mem_pct_max", value: float(0)},
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "mem_limit", value: float(0)},
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "netrx", value: float(0)},
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "nettx", value: float(0)},
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "cpu_max", value: float(250)},
		{container: "nonApp", image: "tsuru/python", hostname: id0, key: "mem_max", value: float(0)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "cpu_max", value: float(250)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "mem_max", value: float(0)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "mem_pct_max", value: float(0)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "mem_limit", value: float(0)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "netrx", value: float(0)},
		{container: "test", image: "tsuru/python", app: "someapp", hostname: id1, key: "nettx", value: float(0)},
	}
	sort.Sort(fakeStatList(expected))
	sort.Sort(fakeStatList(fakeBackend.stats))
	c.Assert(fakeBackend.stats, check.DeepEquals, expected)
}

func (s *S) TestGetMetricsEmpty(c *check.C) {
	var containers []docker.APIContainers
	r := &Reporter{}
	r.getMetrics(containers, []string{})
}

func (s *S) TestSendHostMetrics(c *check.C) {
	r := Reporter{backend: &fakeBackend}
	metrics := map[string]float{"cpu": float(900), "mem": float(512)}
	hostInfo := HostInfo{Name: "hostname"}
	err := r.sendHostMetrics(hostInfo, metrics)
	c.Assert(err, check.IsNil)
	expected := []fakeStat{
		{app: "sysapp", hostname: "hostname", process: "-", key: "cpu", value: float(900)},
		{app: "sysapp", hostname: "hostname", process: "-", key: "mem", value: float(512)},
	}
	if fakeBackend.stats[0].key != "cpu" {
		expected[0], expected[1] = expected[1], expected[0]
	}
	c.Assert(fakeBackend.stats, check.DeepEquals, expected)
}

func (s *S) TestSendHostMetricsFailure(c *check.C) {
	r := Reporter{backend: &fakeBackend}
	prepErr := errors.New("something wen wrong")
	fakeBackend.prepareFailure(prepErr)
	metrics := map[string]float{"cpu": float(900)}
	hostInfo := HostInfo{Name: "hostname"}
	err := r.sendHostMetrics(hostInfo, metrics)
	c.Assert(err, check.Equals, prepErr)
}

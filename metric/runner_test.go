// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"net/http"
	"os"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/testing"
	"gopkg.in/check.v1"
)

type bogusContainer struct {
	config docker.Config
	state  docker.State
	name   string
}

func (s *S) TestRunner(c *check.C) {
	os.Unsetenv("CONTAINER_SELECTION_ENV")
	bogusContainers := s.buildContainers()
	dockerServer, conts := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	s.prepareStats(dockerServer, conts)
	r := NewRunner(dockerServer.URL(), time.Second, "fake")
	err := r.Start()
	c.Assert(err, check.IsNil)
	r.Stop()
	cpuStat := make([]*fakeStat, 0)
	for i, stat := range fakeStatter.stats {
		if stat.key == "cpu_max" {
			cpuStat = append(cpuStat, &fakeStatter.stats[i])
		}
	}
	c.Assert(len(cpuStat), check.Equals, 2)
	expected := []*fakeStat{
		{
			container: "nonApp",
			image:     "tsuru/python",
			hostname:  conts[0].ID[:12],
			key:       "cpu_max",
			value:     float(250),
		},
		{
			container: "app",
			app:       "someapp",
			image:     "tsuru/python",
			hostname:  conts[1].ID[:12],
			process:   "myprocess",
			key:       "cpu_max",
			value:     float(250),
		},
	}
	if cpuStat[0].hostname != conts[0].ID[:12] {
		expected[0], expected[1] = expected[1], expected[0]
	}
	for i, stat := range cpuStat {
		c.Assert(stat, check.DeepEquals, expected[i])
	}
}

func (s *S) TestRunnerSelectionEnv(c *check.C) {
	os.Setenv("CONTAINER_SELECTION_ENV", "TSURU_APPNAME")
	defer os.Unsetenv("CONTAINER_SELECTION_ENV")
	bogusContainers := s.buildContainers()
	dockerServer, conts := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	s.prepareStats(dockerServer, conts)
	r := NewRunner(dockerServer.URL(), time.Second, "fake")
	err := r.Start()
	c.Assert(err, check.IsNil)
	r.Stop()
	cpuStat := make([]*fakeStat, 0)
	for i, stat := range fakeStatter.stats {
		if stat.key == "cpu_max" {
			cpuStat = append(cpuStat, &fakeStatter.stats[i])
		}
	}
	c.Assert(len(cpuStat), check.Equals, 1)
	expected := []*fakeStat{
		{
			container: "app",
			app:       "someapp",
			image:     "tsuru/python",
			hostname:  conts[1].ID[:12],
			process:   "myprocess",
			key:       "cpu_max",
			value:     float(250),
		},
	}
	c.Assert(cpuStat[0], check.DeepEquals, expected[0])
}

func (s *S) startDockerServer(containers []bogusContainer, hook func(*http.Request), c *check.C) (*testing.DockerServer, []docker.Container) {
	server, err := testing.NewServer("127.0.0.1:0", nil, hook)
	c.Assert(err, check.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, check.IsNil)
	createdContainers := make([]docker.Container, len(containers))
	for i, bogus := range containers {
		pullOpts := docker.PullImageOptions{Repository: bogus.config.Image}
		err = client.PullImage(pullOpts, docker.AuthConfiguration{})
		c.Assert(err, check.IsNil)
		createOpts := docker.CreateContainerOptions{Name: bogus.name, Config: &bogus.config}
		container, err := client.CreateContainer(createOpts)
		c.Assert(err, check.IsNil)
		err = server.MutateContainer(container.ID, bogus.state)
		c.Assert(err, check.IsNil)
		createdContainers[i] = *container
	}
	return server, createdContainers
}

func (s *S) buildContainers() []bogusContainer {
	return []bogusContainer{
		{
			name:   "CnonApp",
			config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/"}},
			state:  docker.State{Running: true},
		},
		{
			name:   "Capp",
			config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp", "TSURU_PROCESSNAME=myprocess"}},
			state:  docker.State{Running: true},
		},
		{
			name:   "Ctest",
			config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}},
			state:  docker.State{Running: false, ExitCode: -1},
		},
	}
}

func (s *S) prepareStats(dockerServer *testing.DockerServer, containers []docker.Container) {
	for _, container := range containers {
		dockerServer.PrepareStats(container.ID, func(id string) docker.Stats {
			s := docker.Stats{}
			s.PreCPUStats.CPUUsage.TotalUsage = 50
			s.PreCPUStats.SystemCPUUsage = 20
			s.CPUStats.CPUUsage.TotalUsage = 100
			s.CPUStats.SystemCPUUsage = 40
			s.CPUStats.CPUUsage.PercpuUsage = []uint64{100}
			return s
		})
	}
}

// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"os"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

func (s S) TestWritableCheckRun(c *check.C) {
	dir, err := os.Getwd()
	c.Assert(err, check.IsNil)
	wCheck := writableCheck{path: dir}
	err = wCheck.Run()
	c.Assert(err, check.IsNil)
}

func (s S) TestWritableCheckRunInvalidPath(c *check.C) {
	wCheck := writableCheck{path: "/some/invalid/dir/dont/create/it"}
	err := wCheck.Run()
	c.Assert(err, check.ErrorMatches, "open /some/invalid/dir/dont/create/it/tsuru-bs-ro.check: no such file or directory")
}

func (s S) TestCreateContainerCheckRun(c *check.C) {
	baseConts := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
	}
	dockerServer, conts := s.startDockerServer(baseConts, nil, c)
	client, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	done := make(chan bool)
	go func() {
		defer close(done)
	out:
		for {
			conts, err := client.ListContainers(docker.ListContainersOptions{})
			c.Assert(err, check.IsNil)
			for _, co := range conts {
				if co.ID != conts[0].ID {
					err = client.StopContainer(co.ID, 10)
					break out
				}
			}
		}
	}()
	contCheck := createContainerCheck{
		client:     client,
		baseContID: conts[0].ID,
		message:    "Container is running\nWhat happened?\nSomething happened\n",
	}
	err = contCheck.Run()
	c.Assert(err, check.IsNil)
	<-done
}

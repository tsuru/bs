// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"os"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

func (s S) TestNewCheckCollection(c *check.C) {
	checkColl, err := NewCheckCollection(nil)
	c.Assert(err, check.IsNil)
	c.Assert(checkColl.checks, check.HasLen, 2)
	writableCheck := checkColl.checks["writableRoot"].(*writableCheck)
	ccCheck := checkColl.checks["createContainer"].(*createContainerCheck)
	c.Assert(writableCheck.path, check.Equals, "/")
	c.Assert(ccCheck.baseContID, check.Equals, "")
	c.Assert(ccCheck.client, check.IsNil)
	c.Assert(ccCheck.message, check.Equals, "ok")
}

func (s S) TestNewCheckCollectionExtraPaths(c *check.C) {
	os.Setenv("HOSTCHECK_EXTRA_PATHS", "/var/log, /var/lib/docker")
	defer os.Unsetenv("HOSTCHECK_EXTRA_PATHS")
	checkColl, err := NewCheckCollection(nil)
	c.Assert(err, check.IsNil)
	c.Assert(checkColl.checks, check.HasLen, 4)
	writableCheck1, ok := checkColl.checks["writableCustomPath1"].(*writableCheck)
	c.Assert(ok, check.Equals, true)
	c.Assert(writableCheck1.path, check.Equals, "/var/log")
	writableCheck2, ok := checkColl.checks["writableCustomPath2"].(*writableCheck)
	c.Assert(ok, check.Equals, true)
	c.Assert(writableCheck2.path, check.Equals, "/var/lib/docker")
}

func (s S) TestNewCheckCollectionBaseContainerName(c *check.C) {
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "big-sibling")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	checkColl, err := NewCheckCollection(nil)
	c.Assert(err, check.IsNil)
	c.Assert(checkColl.checks, check.HasLen, 2)
	ccCheck := checkColl.checks["createContainer"].(*createContainerCheck)
	c.Assert(ccCheck.baseContID, check.Equals, "big-sibling")
	c.Assert(ccCheck.client, check.IsNil)
	c.Assert(ccCheck.message, check.Equals, "ok")
}

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
				if co.ID != conts[0].ID && co.Status != "Exit 0" {
					err = client.StopContainer(co.ID, 10)
					break out
				}
			}
		}
	}()
	contCheck := createContainerCheck{
		client:     client,
		baseContID: conts[0].ID,
		message:    "Container is not running\nWhat happened?\nSomething happened\n",
	}
	err = contCheck.Run()
	c.Assert(err, check.IsNil)
	<-done
}

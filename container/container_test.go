// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"net/http"
	"strings"
	"testing"

	"github.com/fsouza/go-dockerclient"
	dTesting "github.com/fsouza/go-dockerclient/testing"
	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func createContainer(c *check.C, url string, envs []string) string {
	dockerClient, err := docker.NewClient(url)
	c.Assert(err, check.IsNil)
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	c.Assert(err, check.IsNil)
	config := docker.Config{
		Image: "myimg",
		Cmd:   []string{"mycmd"},
		Env:   envs,
	}
	opts := docker.CreateContainerOptions{Name: "myContName", Config: &config}
	cont, err := dockerClient.CreateContainer(opts)
	c.Assert(err, check.IsNil)
	return cont.ID
}

func (S) TestInfoClientGetContainer(c *check.C) {
	dockerCalls := 0
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, func(req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/json") {
			dockerCalls++
		}
	})
	c.Assert(err, check.IsNil)
	id := createContainer(c, dockerServer.URL(), []string{"TSURU_PROCESSNAME=procx", "TSURU_APPNAME=coolappname"})
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont, err := client.GetContainer(id)
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(cont.AppName, check.Equals, "coolappname")
	c.Assert(cont.ProcessName, check.Equals, "procx")
	c.Assert(dockerCalls, check.Equals, 1)
	cont, err = client.GetContainer(id)
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 1)
	cached, ok := client.containerCache.Get(id)
	c.Assert(ok, check.Equals, true)
	c.Assert(cached.(*Container), check.DeepEquals, cont)
}

func (S) TestInfoClientGetContainerNoEnvs(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	id := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"})
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	_, err = client.GetContainer(id)
	c.Assert(err, check.Equals, ErrTsuruVariablesNotFound)
}

func (S) TestInfoClientGetContainerNotFound(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	_, err = client.GetContainer("xxxxxx")
	c.Assert(err, check.ErrorMatches, "No such container: xxxxxx")
}

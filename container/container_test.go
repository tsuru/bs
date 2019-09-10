// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"net/http"
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	dTesting "github.com/fsouza/go-dockerclient/testing"
	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func createContainer(c *check.C, url string, envs []string, labels map[string]string, name string) string {
	dockerClient, err := docker.NewClient(url)
	c.Assert(err, check.IsNil)
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	c.Assert(err, check.IsNil)
	config := docker.Config{
		Image:  "myimg",
		Cmd:    []string{"mycmd"},
		Env:    envs,
		Labels: labels,
	}
	opts := docker.CreateContainerOptions{Name: name, Config: &config}
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
	id := createContainer(c, dockerServer.URL(), []string{"TSURU_PROCESSNAME=procx", "TSURU_APPNAME=coolappname"}, nil, "myContName")
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont, err := client.GetContainer(id, true, []string{})
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(cont.AppName, check.Equals, "coolappname")
	c.Assert(cont.ProcessName, check.Equals, "procx")
	c.Assert(dockerCalls, check.Equals, 1)
	cont, err = client.GetContainer(id, true, []string{})
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 1)
	cached, ok := client.containerCache.Get(id)
	c.Assert(ok, check.Equals, true)
	c.Assert(cached.(*Container), check.DeepEquals, cont)
	cont, err = client.GetContainer(id, false, []string{})
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 2)
	cached, ok = client.containerCache.Get(id)
	c.Assert(ok, check.Equals, true)
	c.Assert(cached.(*Container), check.DeepEquals, cont)
}

func (S) TestInfoClientGetContainerNonApp(c *check.C) {
	dockerCalls := 0
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, func(req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/json") {
			dockerCalls++
		}
	})
	c.Assert(err, check.IsNil)
	id := createContainer(c, dockerServer.URL(), nil, nil, "myContName")
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont, err := client.GetContainer(id, true, []string{})
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(cont.AppName, check.Equals, "myContName")
	c.Assert(cont.ProcessName, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 1)
}

func (S) TestInfoClientGetAppContainer(c *check.C) {
	dockerCalls := 0
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, func(req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/json") {
			dockerCalls++
		}
	})
	c.Assert(err, check.IsNil)
	id := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, nil, "myContName")
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont, err := client.GetAppContainer(id, true)
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(cont.AppName, check.Equals, "coolappname")
	c.Assert(dockerCalls, check.Equals, 1)
	cont, err = client.GetAppContainer(id, false)
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 2)
	cached, ok := client.containerCache.Get(id)
	c.Assert(ok, check.Equals, true)
	c.Assert(cached.(*Container), check.DeepEquals, cont)
	cont, _ = client.GetAppContainer(id, true)
	c.Assert(cont.ID, check.Equals, id)
	c.Assert(dockerCalls, check.Equals, 2)
	id = createContainer(c, dockerServer.URL(), []string{""}, nil, "notAnApp")
	cont, err = client.GetAppContainer(id, true)
	c.Assert(err, check.Equals, ErrTsuruVariablesNotFound)
	c.Assert(cont, check.IsNil)
}

func (S) TestInfoClientGetContainerRequiredEnv(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	id := createContainer(c, dockerServer.URL(), []string{"MONITORED=1"}, nil, "myContName")
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	_, err = client.GetContainer(id, true, []string{"NOTMONITORED"})
	c.Assert(err, check.Equals, ErrTsuruVariablesNotFound)
	cont, err := client.GetContainer(id, true, []string{"MONITORED"})
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, id)
}

func (S) TestInfoClientGetContainerNotFound(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	_, err = client.GetContainer("xxxxxx", true, []string{"TSURU_APPNAME"})
	c.Assert(err, check.ErrorMatches, "No such container: xxxxxx")
}

func (S) TestContainerHasEnvs(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	id := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, nil, "myContName")
	c.Assert(err, check.IsNil)
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont, err := client.GetAppContainer(id, false)
	c.Assert(err, check.IsNil)
	c.Assert(cont.HasEnvs([]string{"TSURU_APPNAME"}), check.Equals, true)
	c.Assert(cont.HasEnvs([]string{"ENV"}), check.Equals, false)
	c.Assert(cont.HasEnvs([]string{"TSURU_APPNAME", "ENV"}), check.Equals, false)
}

func (S) TestContainerIsIsolated(c *check.C) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	id1 := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, map[string]string{"is-isolated-run": "true"}, "withTrueLabel")
	id2 := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, map[string]string{"is-isolated-run": "false"}, "withFalseLabel")
	id3 := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, map[string]string{"tsuru.io/is-isolated-run": "true"}, "withTsuruIOLabel")
	id4 := createContainer(c, dockerServer.URL(), []string{"TSURU_APPNAME=coolappname"}, nil, "withoutLabel")
	c.Assert(err, check.IsNil)
	client, err := NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	cont1, err := client.GetAppContainer(id1, false)
	cont2, err := client.GetAppContainer(id2, false)
	cont3, err := client.GetAppContainer(id3, false)
	cont4, err := client.GetAppContainer(id4, false)
	c.Assert(err, check.IsNil)
	c.Assert(cont1.IsIsolated(), check.Equals, true)
	c.Assert(cont2.IsIsolated(), check.Equals, false)
	c.Assert(cont3.IsIsolated(), check.Equals, true)
	c.Assert(cont4.IsIsolated(), check.Equals, false)
}

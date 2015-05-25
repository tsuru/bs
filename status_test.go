// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/testing"
	"gopkg.in/check.v1"
)

func (s S) TestCollectStatus(c *check.C) {
	bogusContainers := []bogusContainer{
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APP_NAME=someapp"}}, state: docker.State{Running: true}},
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APP_NAME=someapp"}}, state: docker.State{Running: false, ExitCode: -1}},
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APP_NAME=someapp"}}, state: docker.State{Running: true, Restarting: true, ExitCode: -1}},
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APP_NAME=someapp"}}, state: docker.State{Running: true}},
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/"}}, state: docker.State{Running: false, ExitCode: 2}},
	}
	dockerServer, containers := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	result := `[{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":false},{"id":"%s","found":false}]`
	buf := bytes.NewBufferString(fmt.Sprintf(result, containers[0].ID, containers[1].ID, containers[2].ID, containers[3].ID, containers[4].ID))
	var resp http.Response
	resp.Body = ioutil.NopCloser(buf)
	resp.Header = make(http.Header)
	resp.Header.Set("Content-Type", "application/json")
	tsuruServer, requests := s.startTsuruServer(&resp)
	defer tsuruServer.Close()
	config.DockerEndpoint = dockerServer.URL()
	config.TsuruEndpoint = tsuruServer.URL
	config.SentinelEnvVar = "TSURU_APP_NAME="
	config.TsuruToken = "some-token"
	collectStatus()
	req := <-requests
	c.Assert(req.request.Header.Get("Authorization"), check.Equals, "bearer some-token")
	c.Assert(req.request.URL.Path, check.Equals, "/units/status")
	c.Assert(req.request.Method, check.Equals, "POST")
	var input []container
	expected := []container{
		{ID: containers[0].ID, Status: "started"},
		{ID: containers[1].ID, Status: "stopped"},
		{ID: containers[2].ID, Status: "error"},
		{ID: containers[3].ID, Status: "started"},
		{ID: containers[4].ID, Status: "stopped"},
	}
	err := json.Unmarshal(req.body, &input)
	c.Assert(err, check.IsNil)
	c.Assert(input, check.DeepEquals, expected)
	dockerClient, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	apiContainers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	c.Assert(err, check.IsNil)
	ids := make([]string, len(apiContainers))
	for i, cont := range apiContainers {
		ids[i] = cont.ID
	}
	c.Assert(ids, check.DeepEquals, []string{containers[0].ID, containers[1].ID, containers[2].ID, containers[4].ID})
}

type tsuruRequest struct {
	request *http.Request
	body    []byte
}

func (S) startTsuruServer(resp *http.Response) (*httptest.Server, <-chan tsuruRequest) {
	reqchan := make(chan tsuruRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := ioutil.ReadAll(r.Body)
		reqchan <- tsuruRequest{request: r, body: b}
		for k, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(k, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}))
	return server, reqchan
}

type bogusContainer struct {
	config docker.Config
	state  docker.State
}

func (S) startDockerServer(containers []bogusContainer, hook func(*http.Request), c *check.C) (*testing.DockerServer, []docker.Container) {
	server, err := testing.NewServer("127.0.0.1:0", nil, hook)
	c.Assert(err, check.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, check.IsNil)
	createdContainers := make([]docker.Container, len(containers))
	for i, bogus := range containers {
		pullOpts := docker.PullImageOptions{Repository: bogus.config.Image}
		err = client.PullImage(pullOpts, docker.AuthConfiguration{})
		c.Assert(err, check.IsNil)
		createOpts := docker.CreateContainerOptions{Config: &bogus.config}
		container, err := client.CreateContainer(createOpts)
		c.Assert(err, check.IsNil)
		err = server.MutateContainer(container.ID, bogus.state)
		c.Assert(err, check.IsNil)
		createdContainers[i] = *container
	}
	return server, createdContainers
}

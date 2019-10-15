// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajg/form"
	docker "github.com/fsouza/go-dockerclient"
	dtesting "github.com/fsouza/go-dockerclient/testing"
	"github.com/tsuru/bs/bslog"
	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (s S) TestReportStatus(c *check.C) {
	var logOutput bytes.Buffer
	bslog.Logger = log.New(&logOutput, "", 0)
	defer func() { bslog.Logger = log.New(os.Stderr, "", log.LstdFlags) }()
	bogusContainers := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true}},
		{name: "x2", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: false, ExitCode: -1, StartedAt: time.Now().Add(-time.Hour)}},
		{name: "x3", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true, Restarting: true, ExitCode: -1}},
		{name: "x4", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true}},
		{name: "x5", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/"}}, state: docker.State{Running: false, ExitCode: 2}},
		{name: "x6", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: false}},
		{name: "x7", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}, Labels: map[string]string{"is-isolated-run": "true"}}, state: docker.State{Running: true}},
		{name: "x8", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}, Labels: map[string]string{"is-isolated-run": "true"}}, state: docker.State{Running: false}},
		{name: "x9", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}, Labels: map[string]string{"is-isolated-run": "false"}}, state: docker.State{Running: true}},
	}
	dockerServer, containers := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	result := `[{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":false},{"id":"%s","found":true},{"id":"%s","found":true}]`
	buf := bytes.NewBufferString(fmt.Sprintf(result, containers[0].ID, containers[1].ID, containers[2].ID, containers[3].ID, containers[5].ID, containers[8].ID))
	var resp http.Response
	resp.StatusCode = http.StatusOK
	resp.Body = ioutil.NopCloser(buf)
	resp.Header = make(http.Header)
	resp.Header.Set("Content-Type", "application/json")
	tsuruServer, requests := s.startTsuruServer(&resp)
	defer tsuruServer.Close()
	reporter, err := NewReporter(&ReporterConfig{
		Interval:       10 * time.Minute,
		DockerEndpoint: dockerServer.URL(),
		TsuruEndpoint:  tsuruServer.URL,
		TsuruToken:     "some-token",
	})
	c.Assert(err, check.IsNil)
	reporter.Stop()
	reporter.reportStatus()
	c.Log(logOutput.String())
	req := <-requests
	c.Assert(req.request.Header.Get("Authorization"), check.Equals, "bearer some-token")
	c.Assert(req.request.Header.Get("Content-Type"), check.Equals, "application/x-www-form-urlencoded")
	c.Assert(req.request.URL.Path, check.Equals, "/node/status")
	c.Assert(req.request.Method, check.Equals, "POST")
	expected := hostStatus{
		Units: []containerStatus{
			{ID: containers[0].ID, Status: "started", Name: "x1"},
			{ID: containers[1].ID, Status: "stopped", Name: "x2"},
			{ID: containers[2].ID, Status: "error", Name: "x3"},
			{ID: containers[3].ID, Status: "started", Name: "x4"},
			{ID: containers[5].ID, Status: "created", Name: "x6"},
			{ID: containers[8].ID, Status: "started", Name: "x9"},
		},
	}
	var input hostStatus
	err = form.DecodeString(&input, string(req.body))
	c.Assert(err, check.IsNil)
	c.Assert(input.Checks, check.HasLen, 3)
	c.Assert(len(input.Addrs) > 0, check.Equals, true)
	input.Checks = nil
	input.Addrs = nil
	sort.Slice(input.Units, func(i, j int) bool {
		return input.Units[i].Name < input.Units[j].Name
	})
	c.Assert(input, check.DeepEquals, expected)
	reporter.waitPendingRemovals()
	dockerClient, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	apiContainers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	c.Assert(err, check.IsNil)
	ids := make([]string, len(apiContainers))
	for i, cont := range apiContainers {
		println(cont.ID)
		ids[i] = cont.ID
	}
	expectedIDs := []string{containers[0].ID, containers[1].ID, containers[2].ID, containers[4].ID, containers[5].ID, containers[6].ID, containers[7].ID, containers[8].ID}
	sort.Strings(ids)
	sort.Strings(expectedIDs)
	c.Assert(ids, check.DeepEquals, expectedIDs)
}

func (s S) TestReportStatus404OnHostStatus(c *check.C) {
	var logOutput bytes.Buffer
	bslog.Logger = log.New(&logOutput, "", 0)
	defer func() { bslog.Logger = log.New(os.Stderr, "", log.LstdFlags) }()
	bogusContainers := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true}},
		{name: "x2", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: false, ExitCode: -1, StartedAt: time.Now().Add(-time.Hour)}},
		{name: "x3", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true, Restarting: true, ExitCode: -1}},
		{name: "x4", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true}},
		{name: "x5", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/"}}, state: docker.State{Running: false, ExitCode: 2}},
		{name: "x6", config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: false}},
	}
	dockerServer, containers := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	result := `[{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":true},{"id":"%s","found":false},{"id":"%s","found":true}]`
	buf := bytes.NewBufferString(fmt.Sprintf(result, containers[0].ID, containers[1].ID, containers[2].ID, containers[3].ID, containers[5].ID))
	var resp http.Response
	resp.StatusCode = http.StatusOK
	resp.Body = ioutil.NopCloser(buf)
	resp.Header = make(http.Header)
	resp.Header.Set("Content-Type", "application/json")
	tsuruServer, requests := s.startTsuruServer(func(req *http.Request) *http.Response {
		if req.URL.Path != "/units/status" {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
				Body:       ioutil.NopCloser(&bytes.Buffer{}),
			}
		}
		return &resp
	})
	defer tsuruServer.Close()
	reporter, err := NewReporter(&ReporterConfig{
		Interval:       10 * time.Minute,
		DockerEndpoint: dockerServer.URL(),
		TsuruEndpoint:  tsuruServer.URL,
		TsuruToken:     "some-token",
	})
	c.Assert(err, check.IsNil)
	reporter.Stop()
	reporter.reportStatus()
	c.Log(logOutput.String())
	reqInvalid := <-requests
	c.Assert(reqInvalid.request.Method, check.Equals, "POST")
	c.Assert(reqInvalid.request.URL.Path, check.Equals, "/node/status")
	req := <-requests
	c.Assert(req.request.Header.Get("Authorization"), check.Equals, "bearer some-token")
	c.Assert(req.request.URL.Path, check.Equals, "/units/status")
	c.Assert(req.request.Method, check.Equals, "POST")
	var input []containerStatus
	expected := []containerStatus{
		{ID: containers[0].ID, Status: "started", Name: "x1"},
		{ID: containers[1].ID, Status: "stopped", Name: "x2"},
		{ID: containers[2].ID, Status: "error", Name: "x3"},
		{ID: containers[3].ID, Status: "started", Name: "x4"},
		{ID: containers[5].ID, Status: "created", Name: "x6"},
	}
	err = json.Unmarshal(req.body, &input)
	c.Assert(err, check.IsNil)
	sort.Slice(input, func(i, j int) bool {
		return input[i].Name < input[j].Name
	})
	c.Assert(input, check.DeepEquals, expected)
	reporter.waitPendingRemovals()
	dockerClient, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	apiContainers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	c.Assert(err, check.IsNil)
	ids := make([]string, len(apiContainers))
	for i, cont := range apiContainers {
		ids[i] = cont.ID
	}
	expectedIDs := []string{containers[0].ID, containers[1].ID, containers[2].ID, containers[4].ID, containers[5].ID}
	sort.Strings(ids)
	sort.Strings(expectedIDs)
	c.Assert(ids, check.DeepEquals, expectedIDs)
}

func (s S) TestReportStatusMultipleRemovals(c *check.C) {
	bogusContainers := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
		{name: "x2", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
		{name: "x3", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
	}
	toBlock := make(chan bool)
	var deleteCount int32
	dockerServer, conts := s.startDockerServer(bogusContainers, func(r *http.Request) {
		if r.Method == "DELETE" {
			atomic.AddInt32(&deleteCount, 1)
			<-toBlock
		}
	}, c)
	defer dockerServer.Stop()
	var statusResp []respUnit
	for _, cont := range conts {
		statusResp = append(statusResp, respUnit{ID: cont.ID, Found: false})
	}
	data, err := json.Marshal(statusResp)
	c.Assert(err, check.IsNil)
	var resp http.Response
	resp.StatusCode = http.StatusOK
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	resp.Header = make(http.Header)
	resp.Header.Set("Content-Type", "application/json")
	tsuruServer, _ := s.startTsuruServer(&resp)
	defer tsuruServer.Close()
	reporter, err := NewReporter(&ReporterConfig{
		Interval:       10 * time.Minute,
		DockerEndpoint: dockerServer.URL(),
		TsuruEndpoint:  tsuruServer.URL,
		TsuruToken:     "some-token",
	})
	c.Assert(err, check.IsNil)
	reporter.Stop()
	reporter.reportStatus()
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	reporter.reportStatus()
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	reporter.reportStatus()
	close(toBlock)
	reporter.waitPendingRemovals()
	c.Assert(deleteCount, check.Equals, int32(3))
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(data))
	reporter.reportStatus()
	reporter.waitPendingRemovals()
	c.Assert(deleteCount, check.Equals, int32(6))
	dockerClient, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	apiContainers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	c.Assert(err, check.IsNil)
	c.Assert(apiContainers, check.HasLen, 0)
}

type tsuruRequest struct {
	request *http.Request
	body    []byte
}

func (S) startTsuruServer(respOrFunc interface{}) (*httptest.Server, <-chan tsuruRequest) {
	reqchan := make(chan tsuruRequest, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		b, _ := ioutil.ReadAll(r.Body)
		reqchan <- tsuruRequest{request: r, body: b}
		var resp *http.Response
		switch v := respOrFunc.(type) {
		case *http.Response:
			resp = v
		case func(*http.Request) *http.Response:
			resp = v(r)
		}
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
	name   string
}

func (S) startDockerServer(containers []bogusContainer, hook func(*http.Request), c *check.C) (*dtesting.DockerServer, []docker.Container) {
	server, err := dtesting.NewServer("127.0.0.1:0", nil, hook)
	c.Assert(err, check.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, check.IsNil)
	createdContainers := make([]docker.Container, len(containers))
	for i, bogus := range containers {
		pullOpts := docker.PullImageOptions{Repository: bogus.config.Image}
		err = client.PullImage(pullOpts, docker.AuthConfiguration{})
		c.Assert(err, check.IsNil)
		createOpts := docker.CreateContainerOptions{Config: &bogus.config, Name: bogus.name}
		container, err := client.CreateContainer(createOpts)
		c.Assert(err, check.IsNil)
		err = server.MutateContainer(container.ID, bogus.state)
		c.Assert(err, check.IsNil)
		createdContainers[i] = *container
	}
	return server, createdContainers
}

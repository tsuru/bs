// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"io/ioutil"
	"os"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

func (s S) TestNewCheckCollection(c *check.C) {
	checkColl := NewCheckCollection(nil)
	c.Assert(checkColl.checks, check.DeepEquals, []hostCheck{
		&writableCheck{path: "/"},
		&createContainerCheck{message: "ok"},
	})
}

func (s S) TestNewCheckCollectionExtraPaths(c *check.C) {
	os.Setenv("HOSTCHECK_EXTRA_PATHS", "/var/log, /var/lib/docker")
	defer os.Unsetenv("HOSTCHECK_EXTRA_PATHS")
	checkColl := NewCheckCollection(nil)
	c.Assert(checkColl.checks, check.DeepEquals, []hostCheck{
		&writableCheck{path: "/"},
		&createContainerCheck{message: "ok"},
		&writableCheck{path: "/var/log"},
		&writableCheck{path: "/var/lib/docker"},
	})
}

func (s S) TestNewCheckCollectionBaseContainerName(c *check.C) {
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "big-sibling")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	checkColl := NewCheckCollection(nil)
	c.Assert(checkColl.checks, check.DeepEquals, []hostCheck{
		&writableCheck{path: "/"},
		&createContainerCheck{message: "ok", baseContID: "big-sibling"},
	})
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
	done := stopContainer(client, conts[0].ID, c)
	contCheck := createContainerCheck{
		client:     client,
		baseContID: conts[0].ID,
		message:    "Container is not running\nWhat happened?\nSomething happened\n",
	}
	err = contCheck.Run()
	c.Assert(err, check.IsNil)
	<-done
}

func (s S) TestCheckCollectionRun(c *check.C) {
	baseConts := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
	}
	dockerServer, conts := s.startDockerServer(baseConts, nil, c)
	client, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	tmpdir, err := ioutil.TempDir("", "hostcheck")
	c.Assert(err, check.IsNil)
	done := stopContainer(client, conts[0].ID, c)
	defer os.RemoveAll(tmpdir)
	os.Setenv("HOSTCHECK_ROOT_PATH_OVERRIDE", tmpdir)
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "x1")
	os.Setenv("HOSTCHECK_CONTAINER_MESSAGE", "Container is not running\nWhat happened?\nSomething happened\n")
	defer os.Unsetenv("HOSTCHECK_ROOT_PATH_OVERRIDE")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	defer os.Unsetenv("HOSTCHECK_CONTAINER_MESSAGE")
	checkColl := NewCheckCollection(client)
	results := checkColl.Run()
	c.Assert(results, check.DeepEquals, []hostCheckResult{
		{Name: "writablePath-" + tmpdir, Successful: true},
		{Name: "createContainer", Successful: true},
	})
	<-done
}

func (s S) TestCheckCollectionRunFilterAll(c *check.C) {
	baseConts := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
	}
	dockerServer, conts := s.startDockerServer(baseConts, nil, c)
	client, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	tmpdir, err := ioutil.TempDir("", "hostcheck")
	c.Assert(err, check.IsNil)
	done := stopContainer(client, conts[0].ID, c)
	defer os.RemoveAll(tmpdir)
	os.Setenv("HOSTCHECK_ROOT_PATH_OVERRIDE", tmpdir)
	os.Setenv("HOSTCHECK_KIND_FILTER", "writablePath, createContainer")
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "x1")
	os.Setenv("HOSTCHECK_CONTAINER_MESSAGE", "Container is not running\nWhat happened?\nSomething happened\n")
	defer os.Unsetenv("HOSTCHECK_ROOT_PATH_OVERRIDE")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	defer os.Unsetenv("HOSTCHECK_CONTAINER_MESSAGE")
	checkColl := NewCheckCollection(client)
	results := checkColl.Run()
	c.Assert(results, check.DeepEquals, []hostCheckResult{
		{Name: "writablePath-" + tmpdir, Successful: true},
		{Name: "createContainer", Successful: true},
	})
	<-done
}

func (s S) TestCheckCollectionRunFilterSingle(c *check.C) {
	dockerServer, _ := s.startDockerServer(nil, nil, c)
	client, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	tmpdir, err := ioutil.TempDir("", "hostcheck")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpdir)
	os.Setenv("HOSTCHECK_ROOT_PATH_OVERRIDE", tmpdir)
	os.Setenv("HOSTCHECK_KIND_FILTER", "writablePath")
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "x1")
	os.Setenv("HOSTCHECK_CONTAINER_MESSAGE", "Container is not running\nWhat happened?\nSomething happened\n")
	defer os.Unsetenv("HOSTCHECK_ROOT_PATH_OVERRIDE")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	defer os.Unsetenv("HOSTCHECK_CONTAINER_MESSAGE")
	defer os.Unsetenv("HOSTCHECK_KIND_FILTER")
	checkColl := NewCheckCollection(client)
	results := checkColl.Run()
	c.Assert(results, check.DeepEquals, []hostCheckResult{
		{Name: "writablePath-" + tmpdir, Successful: true},
	})
}

func (s S) TestCheckCollectionRunTimeout(c *check.C) {
	baseConts := []bogusContainer{
		{name: "x1", config: docker.Config{Image: "tsuru/python"}, state: docker.State{Running: true}},
	}
	dockerServer, _ := s.startDockerServer(baseConts, nil, c)
	client, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	tmpdir, err := ioutil.TempDir("", "hostcheck")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tmpdir)
	os.Setenv("HOSTCHECK_TIMEOUT", "1")
	os.Setenv("HOSTCHECK_ROOT_PATH_OVERRIDE", tmpdir)
	os.Setenv("HOSTCHECK_BASE_CONTAINER_NAME", "x1")
	os.Setenv("HOSTCHECK_CONTAINER_MESSAGE", "Container is not running\nWhat happened?\nSomething happened\n")
	defer os.Unsetenv("HOSTCHECK_TIMEOUT")
	defer os.Unsetenv("HOSTCHECK_ROOT_PATH_OVERRIDE")
	defer os.Unsetenv("HOSTCHECK_BASE_CONTAINER_NAME")
	defer os.Unsetenv("HOSTCHECK_CONTAINER_MESSAGE")
	checkColl := NewCheckCollection(client)
	results := checkColl.Run()
	resultsMap := map[string]hostCheckResult{}
	for _, result := range results {
		resultsMap[result.Name] = result
	}
	c.Assert(resultsMap, check.DeepEquals, map[string]hostCheckResult{
		"writablePath-" + tmpdir: {Name: "writablePath-" + tmpdir, Err: "", Successful: true},
		"createContainer":        {Name: "createContainer", Err: "[host check] timeout running \"createContainer\" check", Successful: false},
	})
}

func (s S) TestParseContainerID(c *check.C) {
	tests := []struct {
		data     string
		expected string
		err      string
	}{
		{
			data: `11:freezer:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
10:cpuacct,cpu:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
9:pids:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
8:blkio:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
7:memory:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
6:cpuset:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
5:devices:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
4:perf_event:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
3:net_prio,net_cls:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
2:hugetlb:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
1:name=systemd:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad`,
			expected: "3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad",
		},
		{
			data: `11:name=systemd:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
10:hugetlb:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
9:perf_event:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
8:blkio:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
7:freezer:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
6:devices:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
5:memory:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
4:cpuacct:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
3:cpu:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
2:cpuset:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804`,
			expected: "6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804",
		},
		{
			data: `11:memory:/user.slice
10:hugetlb:/
9:pids:/user.slice/user-900.slice
8:cpuset:/
7:freezer:/
6:devices:/user.slice
5:cpu,cpuacct:/user.slice
4:net_cls,net_prio:/
3:blkio:/user.slice
2:perf_event:/
1:name=systemd:/user.slice/user-900.slice/session-22.scope`,
			err: "(?s)unable to parse container id from .*",
		},
		{
			data: `11:memory:/init.scope
10:hugetlb:/
9:pids:/init.scope
8:cpuset:/
7:freezer:/
6:devices:/init.scope
5:cpu,cpuacct:/init.scope
4:net_cls,net_prio:/
3:blkio:/init.scope
2:perf_event:/
1:name=systemd:/init.scope`,
			err: "(?s)unable to parse container id from .*",
		},
	}
	for _, tt := range tests {
		f, err := ioutil.TempFile("", "tsurutests")
		c.Assert(err, check.IsNil)
		defer os.Remove(f.Name())
		f.Write([]byte(tt.data))
		f.Close()
		id, err := parseContainerID(f.Name())
		if tt.err != "" {
			c.Assert(err, check.ErrorMatches, tt.err)
			c.Assert(id, check.Equals, "")
		} else {
			c.Assert(err, check.IsNil)
			c.Assert(id, check.Equals, tt.expected)
		}
	}
}

func stopContainer(client *docker.Client, exceptID string, c *check.C) chan bool {
	done := make(chan bool)
	go func() {
		defer close(done)
	out:
		for {
			listedConts, inErr := client.ListContainers(docker.ListContainersOptions{})
			c.Assert(inErr, check.IsNil)
			for _, co := range listedConts {
				if co.ID != exceptID && co.Status != "Exit 0" {
					inErr = client.StopContainer(co.ID, 10)
					c.Assert(inErr, check.IsNil)
					break out
				}
			}
		}
	}()
	return done
}

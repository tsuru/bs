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
	checkColl := NewCheckCollection(nil)
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
	checkColl := NewCheckCollection(nil)
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
	for _, result := range results {
		c.Assert(result.Err, check.Equals, "")
		c.Assert(result.Successful, check.Equals, true)
	}
	<-done
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
		"writableRoot":    {Name: "writableRoot", Err: "", Successful: true},
		"createContainer": {Name: "createContainer", Err: "[host check] timeout running \"createContainer\" check", Successful: false},
	})
}

func (s S) TestParseContainerID(c *check.C) {
	tests := []struct {
		data     string
		expected string
	}{
		{
			`11:freezer:/kubepods/besteffort/pod1a487934-a2c7-11e7-9ab6-06d3b605db81/3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad
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
			"3e86c741c2c6bd556d5e8a3e5bc56363ca40a81927e6d55c30aefdcea7ca54ad",
		},
		{
			`11:name=systemd:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
10:hugetlb:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
9:perf_event:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
8:blkio:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
7:freezer:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
6:devices:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
5:memory:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
4:cpuacct:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
3:cpu:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804
2:cpuset:/docker/6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804`,
			"6d52b4d36625e83be18320e0ce56304186e205334510131e14c6dc73526f5804",
		},
	}
	for _, tt := range tests {
		f, err := ioutil.TempFile("", "tsurutests")
		c.Assert(err, check.IsNil)
		defer os.Remove(f.Name())
		f.Write([]byte(tt.data))
		f.Close()
		id, err := parseContainerID(f.Name())
		c.Assert(err, check.IsNil)
		c.Assert(id, check.Equals, tt.expected)
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

// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

type hostCheck interface {
	Run() error
}

type checkCollection struct {
	checks  map[string]hostCheck
	timeout time.Duration
}

type hostCheckResult struct {
	Name       string
	Err        string
	Successful bool
}

var cgroupIDRegexp = regexp.MustCompile(`(?ms)/docker/(.*?)$`)

func NewCheckCollection(client *docker.Client) *checkCollection {
	hostCheckTimeout := config.SecondsEnvOrDefault(60, "HOSTCHECK_TIMEOUT")
	baseContainerName := config.StringEnvOrDefault("", "HOSTCHECK_BASE_CONTAINER_NAME")
	checkColl := &checkCollection{
		checks: map[string]hostCheck{
			"writableRoot":    &writableCheck{path: "/"},
			"createContainer": &createContainerCheck{client: client, baseContID: baseContainerName, message: "ok"},
		},
		timeout: hostCheckTimeout,
	}
	extraPaths := config.StringsEnvOrDefault(nil, "HOSTCHECK_EXTRA_PATHS")
	for i, p := range extraPaths {
		checkColl.checks[fmt.Sprintf("writableCustomPath%d", i+1)] = &writableCheck{path: p}
	}
	return checkColl
}

func (c *checkCollection) Run() []hostCheckResult {
	result := make([]hostCheckResult, len(c.checks))
	errCh := make(chan error, 1)
	i := 0
	for name, check := range c.checks {
		checkResult := hostCheckResult{Name: name}
		go func() {
			errCh <- check.Run()
		}()
		var timeoutCh <-chan time.Time
		if c.timeout > 0 {
			timeoutCh = time.After(c.timeout)
		}
		select {
		case err := <-errCh:
			checkResult.Successful = err == nil
			if err != nil {
				bslog.Errorf("[host check] failure running %q check: %s", name, err)
				checkResult.Err = err.Error()
			}
		case <-timeoutCh:
			checkResult.Successful = false
			errMsg := fmt.Sprintf("[host check] timeout running %q check", name)
			bslog.Errorf(errMsg)
			checkResult.Err = errMsg
		}
		result[i] = checkResult
		i++
	}
	return result
}

type writableCheck struct {
	path string
}

func (c *writableCheck) Run() error {
	fileName := strings.Join([]string{
		strings.TrimRight(c.path, string(os.PathSeparator)),
		"tsuru-bs-ro.check",
	}, string(os.PathSeparator))
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer os.Remove(fileName)
	defer file.Close()
	data := []byte("ok")
	n, err := file.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}

	return nil
}

type createContainerCheck struct {
	client     *docker.Client
	baseContID string
	message    string
}

func (c *createContainerCheck) setBaseContainerID() error {
	if c.baseContID != "" {
		return nil
	}
	cgroupFile, err := os.Open("/proc/1/cgroup")
	if err != nil {
		return err
	}
	defer cgroupFile.Close()
	data, err := ioutil.ReadAll(cgroupFile)
	if err != nil {
		return err
	}
	result := cgroupIDRegexp.FindSubmatch(data)
	if len(result) != 2 {
		return fmt.Errorf("unable to parse container id from /proc/1/cgroup, returned data:\n%s", string(data))
	}
	c.baseContID = string(result[1])
	return nil
}

func (c *createContainerCheck) Run() error {
	err := c.setBaseContainerID()
	if err != nil {
		return err
	}
	contName := "bs-hostcheck-container"
	c.client.RemoveContainer(docker.RemoveContainerOptions{ID: contName, Force: true})
	baseContInfo, err := c.client.InspectContainer(c.baseContID)
	if err != nil {
		return err
	}
	opts := docker.CreateContainerOptions{
		Name: "bs-hostcheck-container",
		Config: &docker.Config{
			AttachStdout: true,
			AttachStderr: true,
			Image:        baseContInfo.Image,
			Entrypoint:   []string{},
			Cmd:          []string{"echo", "-n", c.message},
		},
	}
	cont, err := c.client.CreateContainer(opts)
	if err != nil {
		return err
	}
	output := bytes.NewBuffer(nil)
	defer c.client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true})
	attachOptions := docker.AttachToContainerOptions{
		Container:    cont.ID,
		OutputStream: output,
		Stream:       true,
		Stdout:       true,
		Success:      make(chan struct{}),
	}
	waiter, err := c.client.AttachToContainerNonBlocking(attachOptions)
	if err != nil {
		return err
	}
	<-attachOptions.Success
	close(attachOptions.Success)
	err = c.client.StartContainer(cont.ID, nil)
	if err != nil {
		return err
	}
	waiter.Wait()
	if output.String() != c.message {
		return fmt.Errorf("unexpected container response: %q", output.String())
	}
	return nil
}

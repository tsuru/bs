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

	docker "github.com/fsouza/go-dockerclient"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

type hostCheck interface {
	Run() error
	Name() string
	Kind() string
}

type checkCollection struct {
	checks          []hostCheck
	checksFilterSet map[string]struct{}
	timeout         time.Duration
	errChannels     map[string]chan error
}

type hostCheckResult struct {
	Name       string
	Err        string
	Successful bool
}

var cgroupIDRegexp = regexp.MustCompile(`(?ms).*/([a-fA-F0-9]+?)$`)

func NewCheckCollection(client *docker.Client) *checkCollection {
	hostCheckTimeout := config.SecondsEnvOrDefault(0, "HOSTCHECK_TIMEOUT")
	baseContainerName := config.StringEnvOrDefault("", "HOSTCHECK_BASE_CONTAINER_NAME")
	rootPathOverride := config.StringEnvOrDefault("/", "HOSTCHECK_ROOT_PATH_OVERRIDE")
	containerCheckMessage := config.StringEnvOrDefault("ok", "HOSTCHECK_CONTAINER_MESSAGE")
	checksFilter := config.StringsEnvOrDefault(nil, "HOSTCHECK_KIND_FILTER")
	var checksFilterSet map[string]struct{}
	if len(checksFilter) > 0 {
		checksFilterSet = make(map[string]struct{})
		for _, kind := range checksFilter {
			checksFilterSet[kind] = struct{}{}
		}
	}
	checkColl := &checkCollection{
		checks: []hostCheck{
			&writableCheck{path: rootPathOverride},
			&createContainerCheck{client: client, baseContID: baseContainerName, message: containerCheckMessage},
		},
		checksFilterSet: checksFilterSet,
		timeout:         hostCheckTimeout,
		errChannels:     make(map[string]chan error),
	}
	extraPaths := config.StringsEnvOrDefault(nil, "HOSTCHECK_EXTRA_PATHS")
	for _, p := range extraPaths {
		checkColl.checks = append(checkColl.checks, &writableCheck{path: p})
	}
	return checkColl
}

func (c *checkCollection) Run() []hostCheckResult {
	result := make([]hostCheckResult, 0, len(c.checks))
	for _, check := range c.checks {
		if c.checksFilterSet != nil {
			if _, inSet := c.checksFilterSet[check.Kind()]; !inSet {
				continue
			}
		}
		name := check.Name()
		checkResult := hostCheckResult{Name: name}
		if c.errChannels[name] == nil {
			c.errChannels[name] = make(chan error)
			go func(hc hostCheck, errCh chan error) {
				errCh <- hc.Run()
			}(check, c.errChannels[name])
		}
		var timeoutCh <-chan time.Time
		if c.timeout > 0 {
			timeoutCh = time.After(c.timeout)
		}
		select {
		case err := <-c.errChannels[name]:
			checkResult.Successful = err == nil
			if err != nil {
				bslog.Errorf("[host check] failure running %q check: %s", name, err)
				checkResult.Err = err.Error()
			}
			c.errChannels[name] = nil
		case <-timeoutCh:
			checkResult.Successful = false
			errMsg := fmt.Sprintf("[host check] timeout running %q check", name)
			bslog.Errorf(errMsg)
			checkResult.Err = errMsg
		}
		result = append(result, checkResult)
	}
	return result
}

type writableCheck struct {
	path string
}

func (c *writableCheck) Kind() string {
	return "writablePath"
}

func (c *writableCheck) Name() string {
	return fmt.Sprintf("writablePath-%s", c.path)
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

func parseContainerID(file string) (string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	result := cgroupIDRegexp.FindSubmatch(data)
	if len(result) != 2 {
		return "", fmt.Errorf("unable to parse container id from %q, returned data:\n%s", file, string(data))
	}
	return string(result[1]), nil
}

func (c *createContainerCheck) setBaseContainerID() error {
	if c.baseContID != "" {
		return nil
	}
	var err error
	c.baseContID, err = parseContainerID("/proc/self/cgroup")
	if err != nil {
		return err
	}
	return nil
}

func (c *createContainerCheck) Kind() string {
	return "createContainer"
}

func (c *createContainerCheck) Name() string {
	return "createContainer"
}

func (c *createContainerCheck) Run() error {
	err := c.setBaseContainerID()
	if err != nil {
		return err
	}
	contName := "bs-hostcheck-container"
	removeErr := c.client.RemoveContainer(docker.RemoveContainerOptions{ID: contName, Force: true})
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
		if err == docker.ErrContainerAlreadyExists && removeErr != nil {
			return fmt.Errorf("failed to remove old container: %v", removeErr)
		}
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

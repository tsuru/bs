// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package status

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type hostCheck interface {
	Run() error
}

type checkCollection struct {
	checks map[string]hostCheck
}

type hostCheckResult struct {
	Name       string
	Err        string
	Successful bool
}

func NewCheckCollection(client *docker.Client) (*checkCollection, error) {
	dockerEnv, err := client.Info()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &checkCollection{
		checks: map[string]hostCheck{
			"writableRoot":       &writableCheck{path: "/"},
			"writableLog":        &writableCheck{path: "/var/log"},
			"writableDockerRoot": &writableCheck{path: dockerEnv.Get("DockerRootDir")},
			"createContainer":    &createContainerCheck{client: client, baseContID: hostname, message: "ok"},
		},
	}, nil
}

func (c *checkCollection) Run() []hostCheckResult {
	result := make([]hostCheckResult, len(c.checks))
	i := 0
	for name, c := range c.checks {
		check := hostCheckResult{Name: name}
		err := c.Run()
		check.Successful = err == nil
		if err != nil {
			check.Err = err.Error()
		}
		result[i] = check
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

func (c *createContainerCheck) Run() error {
	baseContInfo, err := c.client.InspectContainer(c.baseContID)
	if err != nil {
		return err
	}
	opts := docker.CreateContainerOptions{
		Config: &docker.Config{
			AttachStdout: true,
			AttachStderr: true,
			Image:        baseContInfo.Image,
			Entrypoint:   []string{"/bin/sh", "-c"},
			Cmd:          []string{"echo", c.message},
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
	}
	waiter, err := c.client.AttachToContainerNonBlocking(attachOptions)
	if err != nil {
		return err
	}
	err = c.client.StartContainer(cont.ID, nil)
	if err != nil {
		return err
	}
	waiter.Wait()
	if output.String() != c.message {
		return fmt.Errorf("unexpected container response: %s", output.String())
	}
	return nil
}

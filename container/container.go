// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"errors"
	"regexp"
	"strings"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	lru "github.com/hashicorp/golang-lru"
)

var (
	ErrTsuruVariablesNotFound = errors.New("could not find wanted envs")

	hexRegex = regexp.MustCompile(`(?i)^[a-f0-9]+$`)

	appNameLabels     = []string{"bs.tsuru.io/log-app-name", "log-app-name", "io.kubernetes.container.name"}
	processNameLabels = []string{"bs.tsuru.io/log-process-name", "log-process-name", "io.kubernetes.pod.name"}
)

const (
	containerIDTrimSize = 12
	labelIsIsolated     = "is-isolated-run"
)

type InfoClient struct {
	endpoint       string
	client         *docker.Client
	containerCache *lru.Cache
}

type Container struct {
	docker.Container
	client        *InfoClient
	TsuruApp      bool
	AppName       string
	ProcessName   string
	ShortHostname string
}

const (
	fullTimeout = 1 * time.Minute
)

func NewClient(endpoint string) (*InfoClient, error) {
	c := InfoClient{endpoint: endpoint}
	var err error
	c.containerCache, err = lru.New(100)
	if err != nil {
		return nil, err
	}
	c.client, err = docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	c.client.SetTimeout(fullTimeout)
	return &c, nil
}

func (c *InfoClient) GetClient() *docker.Client {
	return c.client
}

func (c *InfoClient) ListContainers() ([]docker.APIContainers, error) {
	return c.client.ListContainers(docker.ListContainersOptions{})
}

// GetContainer returns the container with the provided id if the container has the required
// environment variable. It may use a cache to prevent calling the docker api.
func (c *InfoClient) GetContainer(containerId string, useCache bool, requiredEnvs []string) (*Container, error) {
	cont, err := c.getContainer(containerId, useCache)
	if err != nil {
		return nil, err
	}
	if len(requiredEnvs) > 0 {
		if cont.HasEnvs(requiredEnvs) {
			return cont, nil
		}
		return nil, ErrTsuruVariablesNotFound
	}
	return cont, nil
}

// GetAppContainer returns the container with id containerId if that container
// is an tsuru application. It may use a cache to prevent calling the docker api.
func (c *InfoClient) GetAppContainer(containerId string, useCache bool) (*Container, error) {
	return c.GetContainer(containerId, useCache, []string{"TSURU_APPNAME"})
}

func (c *InfoClient) getContainer(containerId string, useCache bool) (*Container, error) {
	if useCache {
		if val, ok := c.containerCache.Get(containerId); ok {
			return val.(*Container), nil
		}
	}
	cont, err := c.client.InspectContainer(containerId)
	if err != nil {
		return nil, err
	}
	contData := Container{Container: *cont, client: c}
	toFill := map[string]*string{
		"TSURU_APPNAME=":     &contData.AppName,
		"TSURU_PROCESSNAME=": &contData.ProcessName,
	}
	for k, v := range toFill {
		for _, env := range cont.Config.Env {
			if strings.HasPrefix(env, k) {
				*v = env[len(k):]
			}
		}
	}
	if contData.AppName == "" {
		name, ok := contData.GetLabelAny(appNameLabels...)
		if !ok {
			name = strings.TrimPrefix(contData.Name, "/")
		}
		process, ok := contData.GetLabelAny(processNameLabels...)
		if !ok {
			process = contData.ID
		}
		contData.AppName = name
		contData.ProcessName = process
	} else {
		contData.TsuruApp = true
	}
	contData.ShortHostname = contData.Config.Hostname
	if hexRegex.MatchString(contData.Config.Hostname) && len(contData.Config.Hostname) > containerIDTrimSize {
		contData.ShortHostname = contData.Config.Hostname[:containerIDTrimSize]
	}
	c.containerCache.Add(containerId, &contData)
	return &contData, nil
}

func (c *Container) Stats() (*docker.Stats, error) {
	statsCh := make(chan *docker.Stats, 1)
	errCh := make(chan error, 1)
	opts := docker.StatsOptions{
		ID:      c.ID,
		Stream:  false,
		Stats:   statsCh,
		Timeout: 10 * time.Second,
	}
	go func() {
		defer close(errCh)
		err := c.client.client.Stats(opts)
		if err != nil {
			errCh <- err
		}
	}()
	err := <-errCh
	if err != nil {
		return nil, err
	}
	return <-statsCh, nil
}

// HasEnvs checks if the container has the requiredEnvs variables set
func (c *Container) HasEnvs(requiredEnvs []string) bool {
	for _, env := range requiredEnvs {
		hasEnv := false
		for _, val := range c.Config.Env {
			if strings.HasPrefix(val, env) {
				hasEnv = true
				break
			}
		}
		if !hasEnv {
			return false
		}
	}
	return true
}

func (c *Container) IsIsolated() bool {
	isIsolated, ok := c.GetLabelAny(labelIsIsolated)
	return ok && isIsolated == "true"
}

// GetLabelAny returns the first label value that exists with given names
func (c *Container) GetLabelAny(names ...string) (string, bool) {
	for _, n := range names {
		if label, ok := c.Config.Labels[n]; ok {
			return label, true
		}
	}
	return "", false
}

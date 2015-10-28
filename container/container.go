// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/golang-lru"
)

var ErrTsuruVariablesNotFound = errors.New("could not find wanted envs")

type InfoClient struct {
	endpoint       string
	client         *docker.Client
	containerCache *lru.Cache
}

type Container struct {
	docker.Container
	client      *InfoClient
	AppName     string
	ProcessName string
}

const (
	dialTimeout = 10 * time.Second
	fullTimeout = 1 * time.Minute
)

var (
	timeoutDialer = &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
	}
	timeoutHttpClient = &http.Client{
		Transport: &http.Transport{
			Dial:                timeoutDialer.Dial,
			TLSHandshakeTimeout: dialTimeout,
		},
		Timeout: fullTimeout,
	}
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
	c.client.HTTPClient = timeoutHttpClient
	c.client.Dialer = timeoutDialer
	return &c, nil
}

func (c *InfoClient) GetClient() *docker.Client {
	return c.client
}

func (c *InfoClient) GetContainer(containerId string) (*Container, error) {
	return c.getContainer(containerId, false, []string{
		"TSURU_APPNAME=",
		"TSURU_PROCESSNAME=",
	})
}

func (c *InfoClient) GetFreshContainer(containerId string) (*Container, error) {
	return c.getContainer(containerId, true, []string{
		"TSURU_APPNAME=",
	})
}

func (c *InfoClient) getContainer(containerId string, refresh bool, wanted []string) (*Container, error) {
	if !refresh {
		if val, ok := c.containerCache.Get(containerId); ok {
			return val.(*Container), nil
		}
	}
	cont, err := c.client.InspectContainer(containerId)
	if err != nil {
		return nil, err
	}
	contData := Container{Container: *cont, client: c}
	toFill := []*string{
		&contData.AppName,
		&contData.ProcessName,
	}
	remaining := len(wanted)
	for _, val := range cont.Config.Env {
		for i := range wanted {
			if *toFill[i] != "" {
				continue
			}
			if strings.HasPrefix(val, wanted[i]) {
				remaining--
				*toFill[i] = val[len(wanted[i]):]
			}
		}
		if remaining == 0 {
			c.containerCache.Add(containerId, &contData)
			return &contData, nil
		}
	}
	return nil, ErrTsuruVariablesNotFound
}

func (c *Container) Stats() (*docker.Stats, error) {
	statsCh := make(chan *docker.Stats, 1)
	errCh := make(chan error, 1)
	opts := docker.StatsOptions{
		ID:      c.ID,
		Stream:  false,
		Stats:   statsCh,
		Timeout: fullTimeout,
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

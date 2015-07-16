// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/golang-lru"
)

type InfoClient struct {
	endpoint       string
	client         *docker.Client
	containerCache *lru.Cache
}

type Container struct {
	*docker.Container
	AppName     string
	ProcessName string
}

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
	return &c, nil
}

func (c *InfoClient) GetContainer(containerId string) (*Container, error) {
	if val, ok := c.containerCache.Get(containerId); ok {
		return val.(*Container), nil
	}
	cont, err := c.client.InspectContainer(containerId)
	if err != nil {
		return nil, err
	}
	contData := Container{Container: cont}
	wanted := []string{
		"TSURU_APPNAME=",
		"TSURU_PROCESSNAME=",
	}
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
	return nil, fmt.Errorf("could not find wanted envs in %s", containerId)
}

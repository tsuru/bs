// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"fmt"

	"github.com/tsuru/bs/container"
)

type ContainerInfo struct {
	Name     string
	Image    string
	Hostname string
	App      string
	Process  string
	Labels   map[string]string
}

func NewContainerInfo(container *container.Container) ContainerInfo {
	var name string
	if container.Name != "" {
		name = container.Name[1:]
	}
	return ContainerInfo{
		Name:     name,
		Image:    container.Config.Image,
		Hostname: container.Config.Hostname,
		Process:  container.ProcessName,
		App:      container.AppName,
		Labels:   container.Config.Labels,
	}
}

type HostInfo struct {
	Name  string
	Addrs []string
}

type backendFactory func() (Backend, error)

var backends = make(map[string]backendFactory)

// Register registers a new Backend
func Register(name string, b backendFactory) {
	backends[name] = b
}

// Get gets the named backend
func Get(name string) (Backend, error) {
	factory, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend: %q.", name)
	}
	r, err := factory()
	if err != nil {
		return nil, err
	}
	return r, nil
}

type Backend interface {
	Send(container ContainerInfo, key string, value interface{}) error
	SendConn(container ContainerInfo, host string) error
	SendHost(host HostInfo, key string, value interface{}) error
}

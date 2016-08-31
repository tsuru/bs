// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import "github.com/tsuru/bs/container"

type ContainerInfo struct {
	Name     string
	Image    string
	Hostname string
	App      string
	Process  string
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

type Backend interface {
	Send(container ContainerInfo, key string, value interface{}) error
	SendConn(container ContainerInfo, host string) error
	SendHost(host HostInfo, key string, value interface{}) error
}

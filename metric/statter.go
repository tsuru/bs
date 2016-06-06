// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import "github.com/tsuru/bs/container"

type ContainerInfo struct {
	name     string
	image    string
	hostname string
	app      string
	process  string
}

func NewContainerInfo(container *container.Container) ContainerInfo {
	var name string
	if container.Name != "" {
		name = container.Name[1:]
	}
	return ContainerInfo{
		name:     name,
		image:    container.Config.Image,
		hostname: container.Config.Hostname,
		process:  container.ProcessName,
		app:      container.AppName,
	}
}

type HostInfo struct {
	Name  string
	Addrs []string
}

type statter interface {
	Send(container ContainerInfo, key string, value interface{}) error
	SendConn(container ContainerInfo, host string) error
	SendHost(host HostInfo, key string, value interface{}) error
}

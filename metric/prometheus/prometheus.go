// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/metric"
)

func new() (metric.Backend, error) {
	return &prometheus{
		FilePath: config.StringEnvOrDefault("", "METRICS_PROMETHEUS_FILEPATH"),
	}, nil
}

type prometheus struct {
	FilePath string
}

func (s *prometheus) Send(container metric.ContainerInfo, key string, value interface{}) error {
	return nil
}

func (s *prometheus) SendConn(container metric.ContainerInfo, host string) error {
	return nil
}

func (s *prometheus) SendHost(host metric.HostInfo, key string, value interface{}) error {
	return nil
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"
	"net"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

func newLogStash() (statter, error) {
	const (
		defaultClient   = "tsuru"
		defaultPort     = "1984"
		defaultHost     = "localhost"
		defaultProtocol = "udp"
	)
	return &logStash{
		Client:   config.StringEnvOrDefault(defaultClient, "METRICS_LOGSTASH_CLIENT"),
		Host:     config.StringEnvOrDefault(defaultHost, "METRICS_LOGSTASH_HOST"),
		Port:     config.StringEnvOrDefault(defaultPort, "METRICS_LOGSTASH_PORT"),
		Protocol: config.StringEnvOrDefault(defaultProtocol, "METRICS_LOGSTASH_PROTOCOL"),
	}, nil
}

type logStash struct {
	Host     string
	Port     string
	Client   string
	Protocol string
}

func (s *logStash) Send(container ContainerInfo, key string, value interface{}) error {
	message := map[string]interface{}{
		"client": s.Client,
		"count":  1,
		"metric": key,
		"value":  value,
	}
	s.appendInfo(message, container)
	return s.send(message)
}

func (s *logStash) SendConn(container ContainerInfo, host string) error {
	message := map[string]interface{}{
		"client":     s.Client,
		"count":      1,
		"metric":     "connection",
		"connection": host,
	}
	s.appendInfo(message, container)
	return s.send(message)
}

func (s *logStash) SendHost(host HostInfo, key string, value interface{}) error {
	message := map[string]interface{}{
		"client": s.Client,
		"count":  1,
		"metric": "host_" + key,
		"value":  value,
		"host":   host.Name,
		"addr":   host.Addrs,
	}
	return s.send(message)
}

func (s *logStash) appendInfo(message map[string]interface{}, container ContainerInfo) {
	message["host"] = container.hostname
	if container.app != "" {
		message["app"] = container.app
		message["process"] = container.process
	} else {
		message["container"] = container.name
		message["image"] = container.image
	}
}

func (s *logStash) send(message map[string]interface{}) error {
	conn, err := net.Dial(s.Protocol, net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	data, err := json.Marshal(message)
	if err != nil {
		bslog.Errorf("unable to marshal metrics data json. Wrote %d bytes before error: %s", err)
		return err
	}
	bytesWritten, err := conn.Write(data)
	if err != nil {
		bslog.Errorf("unable to send metrics to logstash via UDP. Wrote %d bytes before error: %s", bytesWritten, err)
		return err
	}
	return nil
}

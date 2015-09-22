// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"fmt"
	"net"
	"os"
	"time"

	statsdClient "github.com/quipo/statsd"
	"github.com/tsuru/bs/bslog"
)

func newStatsd() (statter, error) {
	var (
		defaultPrefix string = ""
		defaultPort   string = "8125"
		defaultHost   string = "localhost"
	)
	prefix := os.Getenv("METRICS_STATSD_PREFIX")
	if prefix == "" {
		prefix = defaultPrefix
	}
	port := os.Getenv("METRICS_STATSD_PORT")
	if port == "" {
		port = defaultPort
	}
	host := os.Getenv("METRICS_STATSD_HOST")
	if host == "" {
		host = defaultHost
	}
	return &statsd{
		Host:   host,
		Port:   port,
		Prefix: prefix,
	}, nil
}

type statsd struct {
	Host   string
	Port   string
	Prefix string
}

func (s *statsd) Send(app, hostname, process, key, value string) error {
	prefix := fmt.Sprintf("%stsuru.%s.%s", s.Prefix, app, hostname)
	client := statsdClient.NewStatsdClient(net.JoinHostPort(s.Host, s.Port), prefix)
	client.CreateSocket()
	interval := time.Second * 2
	stats := statsdClient.NewStatsdBuffer(interval, client)
	err := stats.Gauge(key, 0.0)
	if err != nil {
		bslog.Errorf("unable to send metrics to statsd via UDP: %s", err)
		return err
	}
	return nil
}

func (s *statsd) SendConn(app, hostname, process, host string) error {
	return s.Send(app, hostname, process, "connection", host)
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"log"
	"net"
	"time"

	statsdClient "github.com/quipo/statsd"
)

type statsd struct {
	Host   string
	Port   string
	Client string
}

func (s *statsd) Send(key, value string) error {
	prefix := "myproject."
	client := statsdClient.NewStatsdClient(net.JoinHostPort(s.Host, s.Port), prefix)
	client.CreateSocket()
	interval := time.Second * 2
	stats := statsdClient.NewStatsdBuffer(interval, client)
	err := stats.Gauge(key, 0.0)
	if err != nil {
		log.Printf("[ERROR] unable to send metrics to statsd via UDP: %s", err)
		return err
	}
	return nil
}

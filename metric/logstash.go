// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"
	"net"
	"os"

	"github.com/tsuru/bs/bslog"
)

func newLogStash() (statter, error) {
	var (
		defaultClient string = "tsuru"
		defaultPort   string = "1984"
		defaultHost   string = "localhost"
	)
	client := os.Getenv("METRICS_LOGSTASH_CLIENT")
	if client == "" {
		client = defaultClient
	}
	port := os.Getenv("METRICS_LOGSTASH_PORT")
	if port == "" {
		port = defaultPort
	}
	host := os.Getenv("METRICS_LOGSTASH_HOST")
	if host == "" {
		host = defaultHost
	}
	return &logStash{
		Client: client,
		Host:   host,
		Port:   port,
	}, nil
}

type logStash struct {
	Host   string
	Port   string
	Client string
}

func (s *logStash) Send(app, hostname, process, key, value string) error {
	conn, err := net.Dial("udp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	message := map[string]string{
		"client":  s.Client,
		"count":   "1",
		"metric":  key,
		"value":   value,
		"app":     app,
		"host":    hostname,
		"process": process,
	}
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

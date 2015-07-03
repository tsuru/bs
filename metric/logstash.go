// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"
	"log"
	"net"
)

type logStash struct {
	Host   string
	Port   string
	Client string
}

func (s *logStash) Send(key, value string) error {
	conn, err := net.Dial("udp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	message := map[string]string{
		"client": s.Client,
		"count":  "1",
		"metric": key,
		"value":  value,
		"app":    "app",
		"host":   "host",
	}
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[ERROR] unable to marshal metrics data json. Wrhote %d bytes before error: %s", err)
		return err
	}
	bytesWritten, err := conn.Write(data)
	if err != nil {
		log.Printf("[ERROR] unable to send metrics to logstash via UDP. Wrhote %d bytes before error: %s", bytesWritten, err)
		return err
	}
	return nil
}

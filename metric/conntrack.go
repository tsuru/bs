// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os/exec"
	"sync/atomic"
)

type conn struct {
	Source      string
	Destination string
}

type connData struct {
	atomic.Value
}

func (d *connData) conns() []conn {
	conns := d.Load()
	return conns.([]conn)
}

var data connData

type conntrackResult struct {
	Items []struct {
		Metas []struct {
			Direction  string `xml:"direction,attr"`
			SourceIP   string `xml:"layer3>src"`
			DestIP     string `xml:"layer3>dst"`
			SourcePort string `xml:"layer4>sport"`
			DestPort   string `xml:"layer4>dport"`
		} `xml:"meta"`
	} `xml:"flow"`
}

func conntrack() error {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("conntrack", "-p", "tcp", "-L", "--state", "ESTABLISHED", "-o", "xml")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("[ERROR] conntrack failed: %s. Output: %s", err, stderr.String())
		return err
	}
	var result conntrackResult
	err = xml.Unmarshal(stdout.Bytes(), &result)
	if err != nil {
		return err
	}
	var conns []conn
	for _, item := range result.Items {
		if len(item.Metas) > 0 {
			if item.Metas[0].SourceIP != "127.0.0.1" && item.Metas[0].DestIP != "127.0.0.1" {
				source := fmt.Sprintf("%s:%s", item.Metas[0].SourceIP, item.Metas[0].SourcePort)
				destination := fmt.Sprintf("%s:%s", item.Metas[0].DestIP, item.Metas[0].DestPort)
				conns = append(conns, conn{Source: source, Destination: destination})
			}
		}
	}
	data.Store(conns)
	return nil
}

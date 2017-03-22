// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"encoding/json"
	"io"
	stdSyslog "log/syslog"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/tsuru/bs/bslog"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

const (
	severityMask = 0x07
	facilityMask = 0xf8
)

type fileMonitor struct {
	handler   syslog.Handler
	cmd       *exec.Cmd
	reader    io.ReadCloser
	container []byte
	finished  int32
}

type logLine struct {
	Log    rawByte
	Stream string
	Time   time.Time
}

type rawByte []byte

func (b *rawByte) UnmarshalText(val []byte) error {
	*b = val
	return nil
}

func newFileMonitor(handler syslog.Handler, path, containerID string) (*fileMonitor, error) {
	m := &fileMonitor{
		cmd:       exec.Command("tail", "-n", "+0", "-F", path),
		handler:   handler,
		container: []byte(containerID),
	}
	var err error
	m.reader, err = m.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *fileMonitor) streamOutput() {
	dec := json.NewDecoder(m.reader)
	var lineData logLine
	for {
		err := dec.Decode(&lineData)
		if err != nil {
			if err != io.EOF {
				bslog.Errorf("error decoding log file line: %v", err)
			}
			return
		}
		facility := stdSyslog.LOG_DAEMON
		severity := stdSyslog.LOG_INFO
		if lineData.Stream != "stdout" {
			severity = stdSyslog.LOG_ERR
		}
		pr := int((facility & facilityMask) | (severity & severityMask))
		m.handler.Handle(format.LogParts{"parts": &rawLogParts{
			content:   bytes.TrimSpace(lineData.Log),
			ts:        lineData.Time,
			priority:  []byte(strconv.Itoa(pr)),
			container: m.container,
		}}, 0, nil)
	}
}

func (m *fileMonitor) alive() bool {
	return atomic.LoadInt32(&m.finished) == 0
}

func (m *fileMonitor) stop() {
	if m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}

func (m *fileMonitor) wait() error {
	return m.cmd.Wait()
}

func (m *fileMonitor) run() error {
	err := m.cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		defer atomic.AddInt32(&m.finished, 1)
		m.streamOutput()
		m.wait()
	}()
	return nil
}

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
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tsuru/bs/bslog"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

const (
	severityMask = 0x07
	facilityMask = 0xf8

	podContainerName    = "POD"
	kubeSystemNamespace = "kube-system"
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

type logFileEntry struct {
	podName       string
	namespace     string
	containerID   string
	containerName string
}

func logEntryFromName(fileName string) logFileEntry {
	entry := logFileEntry{}
	parts := strings.Split(fileName, "_")
	if len(parts) > 0 {
		entry.podName = parts[0]
	}
	if len(parts) > 1 {
		entry.namespace = parts[1]
	}
	if len(parts) > 2 {
		part := strings.TrimSuffix(parts[2], ".log")
		i := strings.LastIndex(part, "-")
		if i != -1 {
			entry.containerName, entry.containerID = part[:i], part[i+1:]
		}
	}
	return entry
}

type kubernetesLogStreamer struct {
	dir      string
	quit     chan struct{}
	monitors map[string]*fileMonitor
	handler  syslog.Handler
}

func newKubeLogStreamer(handler syslog.Handler, dir string) *kubernetesLogStreamer {
	return &kubernetesLogStreamer{
		dir:      dir,
		handler:  handler,
		quit:     make(chan struct{}),
		monitors: make(map[string]*fileMonitor),
	}
}

func (s *kubernetesLogStreamer) stop() {
	s.quit <- struct{}{}
}

func (s *kubernetesLogStreamer) watch() {
	for {
		files, err := filepath.Glob(filepath.Join(s.dir, "*.log"))
		if err != nil {
			bslog.Errorf("unable to list files in directory: %s", err)
		}
		for _, f := range files {
			entry := logEntryFromName(filepath.Base(f))
			if entry.containerName == podContainerName ||
				entry.namespace == kubeSystemNamespace {
				continue
			}
			m := s.monitors[entry.containerID]
			if m != nil && !m.alive() {
				m = nil
			}
			if m == nil {
				m, err = newFileMonitor(s.handler, f, entry.containerID)
				if err != nil {
					bslog.Errorf("unable to create file monitor for %q: %s", f, err)
					continue
				}
				err = m.run()
				if err != nil {
					bslog.Errorf("unable to run file monitor for %q: %s", f, err)
					continue
				}
				s.monitors[entry.containerID] = m
			}
		}
		select {
		case <-time.After(time.Second):
		case <-s.quit:
			for _, m := range s.monitors {
				m.stop()
				m.wait()
			}
			s.monitors = nil
			return
		}
	}
}
// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	stdSyslog "log/syslog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/container"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

const (
	severityMask = 0x07
	facilityMask = 0xf8

	podContainerName    = "POD"
	kubeSystemNamespace = "kube-system"
)

var (
	errNoLogDirectory = errors.New("monitor directory not found")

	updatePosInterval = 5 * time.Second
)

type fileMonitor struct {
	handler        syslog.Handler
	mu             sync.RWMutex
	cmd            *exec.Cmd
	path           string
	finished       bool
	reader         io.ReadCloser
	container      []byte
	streamDone     chan struct{}
	posUpdateDone  chan struct{}
	loadedLastTime int64
	lastTime       int64
	posFile        string
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
		cmd:           exec.Command("tail", "-n", "+0", "-F", path),
		handler:       handler,
		container:     []byte(containerID),
		streamDone:    make(chan struct{}),
		posUpdateDone: make(chan struct{}),
		path:          path,
	}
	var err error
	m.reader, err = m.cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *fileMonitor) loadLastPos() error {
	if m.posFile == "" {
		return nil
	}
	data, err := ioutil.ReadFile(m.posFile)
	if err == nil {
		m.loadedLastTime, _ = strconv.ParseInt(string(data), 10, 64)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *fileMonitor) updatePos() {
	if m.posFile == "" {
		return
	}
	go func() {
		defer close(m.posUpdateDone)
		for {
			select {
			case <-time.After(updatePosInterval):
			case <-m.streamDone:
				return
			}
			t := atomic.LoadInt64(&m.lastTime)
			data := strconv.FormatInt(t, 10)
			err := ioutil.WriteFile(m.posFile, []byte(data), 0600)
			if err != nil {
				bslog.Errorf("error storing last timestamp in %q: %v", m.posFile, err)
			}
		}
	}()
}

func (m *fileMonitor) streamOutput() {
	defer close(m.streamDone)
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
		timeNano := lineData.Time.UnixNano()
		if timeNano <= m.loadedLastTime {
			continue
		}
		facility := stdSyslog.LOG_DAEMON
		severity := stdSyslog.LOG_INFO
		if lineData.Stream != "stdout" {
			severity = stdSyslog.LOG_ERR
		}
		pr := int((facility & facilityMask) | (severity & severityMask))
		atomic.StoreInt64(&m.lastTime, timeNano)
		m.handler.Handle(format.LogParts{"parts": &rawLogParts{
			content:   bytes.TrimSpace(lineData.Log),
			ts:        lineData.Time,
			priority:  []byte(strconv.Itoa(pr)),
			container: m.container,
		}}, 0, nil)
	}
}

func (m *fileMonitor) alive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return !m.finished
}

func (m *fileMonitor) stop() {
	if m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}

func (m *fileMonitor) wait() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finished = true
	<-m.streamDone
	if m.posFile != "" {
		<-m.posUpdateDone
	}
	return m.cmd.Wait()
}

func (m *fileMonitor) start() error {
	err := m.loadLastPos()
	if err != nil {
		return err
	}
	return m.cmd.Start()
}

func (m *fileMonitor) run() {
	m.updatePos()
	go func() {
		m.streamOutput()
		m.wait()
	}()
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
	posDir   string
	quit     chan struct{}
	monitors map[string]*fileMonitor
	handler  syslog.Handler
	client   *container.InfoClient
}

func newKubeLogStreamer(handler syslog.Handler, client *container.InfoClient, dir, posDir string) (*kubernetesLogStreamer, error) {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNoLogDirectory
		}
		return nil, err
	}
	if posDir != "" {
		err = os.MkdirAll(posDir, 0755)
		if err != nil {
			return nil, err
		}
	}
	return &kubernetesLogStreamer{
		dir:      dir,
		posDir:   posDir,
		handler:  handler,
		quit:     make(chan struct{}),
		monitors: make(map[string]*fileMonitor),
		client:   client,
	}, nil
}

func (s *kubernetesLogStreamer) stop() {
	s.quit <- struct{}{}
}

func (s *kubernetesLogStreamer) watchOnce() {
	for id, m := range s.monitors {
		_, err := os.Stat(m.path)
		if err != nil && os.IsNotExist(err) {
			m.stop()
			delete(s.monitors, id)
		}
	}
	files, err := filepath.Glob(filepath.Join(s.dir, "*.log"))
	if err != nil {
		bslog.Errorf("unable to list files in directory: %s", err)
	}
	for _, f := range files {
		fileName := filepath.Base(f)
		entry := logEntryFromName(fileName)
		if entry.containerName == podContainerName ||
			entry.namespace == kubeSystemNamespace {
			continue
		}
		_, err = s.client.GetAppContainer(entry.containerID, true)
		if err != nil {
			if err != container.ErrTsuruVariablesNotFound {
				bslog.Errorf("unable to get container info for %q: %s", f, err)
			}
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
			if s.posDir != "" {
				m.posFile = filepath.Join(s.posDir, fileName+".tsurubs.pos")
			}
			err = m.start()
			if err != nil {
				bslog.Errorf("unable to run file monitor for %q: %s", f, err)
				continue
			}
			s.monitors[entry.containerID] = m
			m.run()
		}
	}
}

func (s *kubernetesLogStreamer) watch() {
	for {
		s.watchOnce()
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

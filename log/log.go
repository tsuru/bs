// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/container"
	"github.com/tsuru/tsuru/app"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

const (
	forwardConnDialTimeout  = time.Second
	forwardConnWriteTimeout = time.Second
	noneBackend             = "none"
)

var (
	stopWg      sync.WaitGroup
	logBackends = map[string]func() logBackend{
		"syslog": func() logBackend { return &syslogBackend{} },
		"tsuru":  func() logBackend { return &tsuruBackend{} },
	}
)

type LogMessage struct {
	logEntry  *app.Applog
	syslogMsg []byte
}

type LogForwarder struct {
	BindAddress     string
	DockerEndpoint  string
	EnabledBackends []string
	infoClient      *container.InfoClient
	server          *syslog.Server
	backends        []logBackend
}

type forwarderBackend interface {
	connect() (net.Conn, error)
	process(conn net.Conn, msg *LogMessage) error
	close(conn net.Conn)
}

type logBackend interface {
	initialize() error
	sendMessage(int, time.Time, string, string, string, string)
	stop()
}

func processMessages(forwarder forwarderBackend, bufferSize int) (chan<- *LogMessage, chan<- bool, error) {
	ch := make(chan *LogMessage, bufferSize)
	quit := make(chan bool)
	if initializable, ok := forwarder.(interface {
		initialize(<-chan bool)
	}); ok {
		initializable.initialize(quit)
	}
	conn, err := forwarder.connect()
	if err != nil {
		return nil, nil, err
	}
	stopWg.Add(1)
	go func() {
		defer stopWg.Done()
		var err error
		for {
			select {
			case <-quit:
				return
			default:
			}
			if conn == nil {
				conn, err = forwarder.connect()
				if err != nil {
					conn = nil
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
		loop:
			for {
				select {
				case <-quit:
					break loop
				case msg := <-ch:
					if msg == nil {
						break loop
					}
					err = forwarder.process(conn, msg)
					if err != nil {
						break loop
					}
				}
			}
			forwarder.close(conn)
			if err == nil {
				break
			}
			bslog.Errorf("[log forwarder] error writing to %#v: %s", forwarder, err)
			conn = nil
		}
	}()
	return ch, quit, nil
}

func (l *LogForwarder) Start() (err error) {
	defer func() {
		if err != nil {
			l.stopWait()
		}
	}()
	if len(l.EnabledBackends) == 1 && l.EnabledBackends[0] == noneBackend {
		return
	}
	for _, backendName := range l.EnabledBackends {
		constructor := logBackends[backendName]
		if constructor == nil {
			return fmt.Errorf("invalid log backend: %s", backendName)
		}
		backend := constructor()
		err = backend.initialize()
		if err != nil {
			return fmt.Errorf("unable to initialize log backend %q: %s", backendName, err)
		}
		l.backends = append(l.backends, backend)
	}
	if len(l.backends) == 0 {
		bslog.Warnf("no log backend enabled, discarding all received log messages.")
	}
	l.infoClient, err = container.NewClient(l.DockerEndpoint)
	if err != nil {
		return
	}
	l.server = syslog.NewServer()
	l.server.SetHandler(l)
	l.server.SetFormat(LenientFormat{})
	url, err := url.Parse(l.BindAddress)
	if err != nil {
		return
	}
	if url.Scheme == "tcp" {
		err = l.server.ListenTCP(url.Host)
	} else if url.Scheme == "udp" {
		err = l.server.ListenUDP(url.Host)
	} else {
		err = fmt.Errorf("invalid protocol %q, expected tcp or udp", url.Scheme)
	}
	if err != nil {
		return
	}
	return l.server.Boot()
}

func (l *LogForwarder) Wait() {
	if l.server != nil {
		l.server.Wait()
	}
	stopWg.Wait()
}

func (l *LogForwarder) Stop() {
	if l.server != nil {
		l.server.Kill()
	}
	for _, backend := range l.backends {
		backend.stop()
	}
}

func (l *LogForwarder) stopWait() {
	l.Stop()
	l.Wait()
}

func (l *LogForwarder) Handle(logParts format.LogParts, msgLen int64, err error) {
	if err != nil {
		bslog.Debugf("[log forwarder] ignored msg %#v error processing: %s", logParts, err)
		return
	}
	content, _ := logParts["content"].(string)
	if content == "" {
		// Silently ignored as docker sometimes will send messages with empty
		// content.
		return
	}
	ts, _ := logParts["timestamp"].(time.Time)
	priority, _ := logParts["priority"].(int)
	if ts.IsZero() || priority == 0 {
		bslog.Debugf("[log forwarder] invalid message %#v", logParts)
		return
	}
	contId, _ := logParts["container_id"].(string)
	if contId == "" {
		contId, _ = logParts["hostname"].(string)
	}
	contData, err := l.infoClient.GetAppContainer(contId, true)
	if err != nil {
		bslog.Debugf("[log forwarder] ignored msg %#v error to get appname: %s", logParts, err)
		return
	}
	for _, backend := range l.backends {
		backend.sendMessage(priority, ts, contId, contData.AppName, contData.ProcessName, content)
	}
}

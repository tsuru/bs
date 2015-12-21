// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeromer/syslogparser"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/container"
	"github.com/tsuru/tsuru/app"
	"golang.org/x/net/websocket"
	"gopkg.in/mcuadros/go-syslog.v2"
)

const (
	forwardConnDialTimeout  = time.Second
	forwardConnWriteTimeout = time.Second
)

var debugStopWg sync.WaitGroup

type LogMessage struct {
	logEntry  *app.Applog
	syslogMsg []byte
}

type LogForwarder struct {
	BufferSize       int
	BindAddress      string
	ForwardAddresses []string
	DockerEndpoint   string
	TsuruEndpoint    string
	TsuruToken       string
	SyslogTimezone   string
	TlsConfig        *tls.Config
	WSPingInterval   time.Duration
	WSPongInterval   time.Duration
	infoClient       *container.InfoClient
	forwardChans     []chan<- *LogMessage
	forwardQuitChans []chan<- bool
	server           *syslog.Server
	syslogLocation   *time.Location
	nextNotify       <-chan time.Time
}

type processable interface {
	connect() (net.Conn, error)
	process(conn net.Conn, msg *LogMessage) error
	close(conn net.Conn)
}

type syslogForwarder struct {
	url *url.URL
}

type wsForwarder struct {
	url          string
	token        string
	tlsConfig    *tls.Config
	connMutex    sync.Mutex
	pingInterval time.Duration
	pongInterval time.Duration
}

func processMessages(processInfo processable, bufferSize int) (chan<- *LogMessage, chan<- bool, error) {
	ch := make(chan *LogMessage, bufferSize)
	quit := make(chan bool)
	conn, err := processInfo.connect()
	if err != nil {
		return nil, nil, err
	}
	debugStopWg.Add(1)
	go func() {
		defer debugStopWg.Done()
		var err error
		for {
			select {
			case <-quit:
				break
			default:
			}
			if conn == nil {
				conn, err = processInfo.connect()
				if err != nil {
					conn = nil
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			for msg := range ch {
				err = processInfo.process(conn, msg)
				if err != nil {
					break
				}
			}
			processInfo.close(conn)
			if err == nil {
				break
			}
			bslog.Errorf("[log forwarder] error writing to %#v: %s", processInfo, err)
			conn = nil
		}
	}()
	return ch, quit, nil
}

func (f *syslogForwarder) connect() (net.Conn, error) {
	conn, err := net.DialTimeout(f.url.Scheme, f.url.Host, forwardConnDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("[log forwarder] unable to connect to %q: %s", f.url, err)
	}
	return conn, nil
}

func (f *syslogForwarder) process(conn net.Conn, msg *LogMessage) error {
	err := conn.SetWriteDeadline(time.Now().Add(forwardConnWriteTimeout))
	if err != nil {
		return err
	}
	n, err := conn.Write(msg.syslogMsg)
	if err != nil {
		return err
	}
	if n < len(msg.syslogMsg) {
		return fmt.Errorf("[log forwarder] short write trying to write log to %q", conn.RemoteAddr())
	}
	return nil
}

func (f *syslogForwarder) close(conn net.Conn) {
	// Reset deadline, if we don't do this the connection remains open
	// on the other end (causing tests to fail) for some weird reason.
	conn.SetWriteDeadline(time.Time{})
	conn.Close()
}

func (f *wsForwarder) connect() (net.Conn, error) {
	config, err := websocket.NewConfig(f.url, "ws://localhost/")
	if err != nil {
		return nil, err
	}
	if f.tlsConfig != nil {
		config.TlsConfig = f.tlsConfig
	}
	config.Header.Add("Authorization", "bearer "+f.token)
	var client net.Conn
	host, port, _ := net.SplitHostPort(config.Location.Host)
	if host == "" {
		host = config.Location.Host
	}
	dialer := &net.Dialer{
		Timeout:   forwardConnDialTimeout,
		KeepAlive: 30 * time.Second,
	}
	switch config.Location.Scheme {
	case "ws":
		if port == "" {
			port = "80"
		}
		client, err = dialer.Dial("tcp", fmt.Sprintf("%s:%s", host, port))
	case "wss":
		if port == "" {
			port = "443"
		}
		client, err = tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%s", host, port), config.TlsConfig)
	default:
		err = websocket.ErrBadScheme
	}
	if err != nil {
		return nil, err
	}
	ws, err := websocket.NewClient(config, client)
	if err != nil {
		client.Close()
		return nil, err
	}
	pingWriter, err := ws.NewFrameWriter(websocket.PingFrame)
	if err != nil {
		client.Close()
		bslog.Errorf("[log forwarder] unable to create ping frame writer, closing websocket: %s", err)
		return nil, err
	}
	lastPongTime := time.Now().UnixNano()
	debugStopWg.Add(2)
	go func() {
		defer debugStopWg.Done()
		defer client.Close()
		for {
			frame, err := ws.NewFrameReader()
			if err != nil {
				bslog.Errorf("[log forwarder] unable to create pong frame reader, closing websocket: %s", err)
				return
			}
			if frame.PayloadType() == websocket.PongFrame {
				atomic.StoreInt64(&lastPongTime, time.Now().UnixNano())
			}
			io.Copy(ioutil.Discard, frame)
		}
	}()
	go func() {
		defer debugStopWg.Done()
		defer client.Close()
		for range time.Tick(f.pingInterval) {
			err := f.writeWithDeadline(ws, pingWriter, []byte{'z'})
			if err != nil {
				bslog.Errorf("[log forwarder] ping: %s", err)
				return
			}
			mylastPongTime := atomic.LoadInt64(&lastPongTime)
			lastPong := time.Unix(0, mylastPongTime)
			now := time.Now()
			if now.After(lastPong.Add(f.pongInterval)) {
				bslog.Errorf("[log forwarder] no pong response in %v, closing websocket", now.Sub(lastPong))
				return
			}
		}
	}()
	return ws, nil
}

func (f *wsForwarder) writeWithDeadline(conn net.Conn, writer io.WriteCloser, data []byte) error {
	f.connMutex.Lock()
	defer f.connMutex.Unlock()
	err := conn.SetWriteDeadline(time.Now().Add(forwardConnWriteTimeout))
	if err != nil {
		return fmt.Errorf("error setting deadline: %s", err)
	}
	n, err := writer.Write(data)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	if n < len(data) {
		return fmt.Errorf("short write trying to write log to %q", conn.RemoteAddr())
	}
	return nil
}

func (f *wsForwarder) process(conn net.Conn, msg *LogMessage) error {
	data, err := json.Marshal(msg.logEntry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	err = f.writeWithDeadline(conn, conn, data)
	if err != nil {
		return err
	}
	return nil
}

func (f *wsForwarder) close(conn net.Conn) {
	// Reset deadline, if we don't do this the connection remains open
	// on the other end (causing tests to fail) for some weird reason.
	f.connMutex.Lock()
	defer f.connMutex.Unlock()
	conn.SetWriteDeadline(time.Time{})
	conn.Close()
}

func (l *LogForwarder) initForwardConnections() error {
	for _, addr := range l.ForwardAddresses {
		forwardUrl, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("unable to parse %q: %s", addr, err)
		}
		forwardChan, quitChan, err := processMessages(&syslogForwarder{
			url: forwardUrl,
		}, l.BufferSize)
		if err != nil {
			return err
		}
		l.forwardChans = append(l.forwardChans, forwardChan)
		l.forwardQuitChans = append(l.forwardQuitChans, quitChan)
	}
	return nil
}

func (l *LogForwarder) initWSConnection() error {
	if l.TsuruEndpoint == "" {
		return nil
	}
	tsuruUrl, err := url.Parse(l.TsuruEndpoint)
	if err != nil {
		return err
	}
	tsuruUrl.Path = "/logs"
	if tsuruUrl.Scheme == "https" {
		tsuruUrl.Scheme = "wss"
	} else {
		tsuruUrl.Scheme = "ws"
	}
	forwardChan, quitChan, err := processMessages(&wsForwarder{
		url:          tsuruUrl.String(),
		token:        l.TsuruToken,
		tlsConfig:    l.TlsConfig,
		pingInterval: l.WSPingInterval,
		pongInterval: l.WSPongInterval,
	}, l.BufferSize)
	if err != nil {
		return err
	}
	l.forwardChans = append(l.forwardChans, forwardChan)
	l.forwardQuitChans = append(l.forwardQuitChans, quitChan)
	return nil
}

func (l *LogForwarder) Start() (err error) {
	defer func() {
		if err != nil {
			l.stop()
		}
	}()
	err = l.initWSConnection()
	if err != nil {
		return
	}
	err = l.initForwardConnections()
	if err != nil {
		return
	}
	l.infoClient, err = container.NewClient(l.DockerEndpoint)
	if err != nil {
		return
	}
	l.nextNotify = time.After(0)
	l.syslogLocation = time.Local
	if l.SyslogTimezone != "" {
		tz, err := time.LoadLocation(l.SyslogTimezone)
		if err == nil {
			l.syslogLocation = tz
		} else {
			bslog.Warnf("unable to parse syslog timezone format: %s", err)
		}
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

func (l *LogForwarder) stop() {
	if l.server != nil {
		l.server.Kill()
	}
	if l.server != nil {
		l.server.Wait()
	}
	for _, ch := range l.forwardQuitChans {
		close(ch)
	}
	for _, ch := range l.forwardChans {
		close(ch)
	}
	debugStopWg.Wait()
}

func (l *LogForwarder) Handle(logParts syslogparser.LogParts, msgLen int64, err error) {
	if err != nil {
		bslog.Debugf("[log forwarder] ignored msg %#v error processing: %s", logParts, err)
		return
	}
	contId, _ := logParts["container_id"].(string)
	if contId == "" {
		contId, _ = logParts["hostname"].(string)
	}
	contData, err := l.infoClient.GetContainer(contId)
	if err != nil {
		bslog.Debugf("[log forwarder] ignored msg %#v error to get appname: %s", logParts, err)
		return
	}
	ts, _ := logParts["timestamp"].(time.Time)
	priority, _ := logParts["priority"].(int)
	content, _ := logParts["content"].(string)
	if ts.IsZero() || priority == 0 || content == "" {
		bslog.Debugf("[log forwarder] invalid message %#v", logParts)
		return
	}
	msg := &LogMessage{
		logEntry: &app.Applog{
			Date:    ts,
			AppName: contData.AppName,
			Message: content,
			Source:  contData.ProcessName,
			Unit:    contId,
		},
		syslogMsg: []byte(fmt.Sprintf("<%d>%s %s %s[%s]: %s\n",
			priority,
			ts.In(l.syslogLocation).Format(time.Stamp),
			contId,
			contData.AppName,
			contData.ProcessName,
			content,
		)),
	}
	for _, ch := range l.forwardChans {
		select {
		case ch <- msg:
		default:
			select {
			case <-l.nextNotify:
				bslog.Errorf("Dropping log messages to due to full channel buffer.")
				l.nextNotify = time.After(time.Minute)
			default:
			}
		}
	}
}

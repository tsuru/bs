// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/tsuru/app"
	"golang.org/x/net/websocket"
)

var (
	// Overridden by tests with tls enabled.
	testTlsConfig *tls.Config

	errConnMaxAgeExceeded = errors.New("max connection age exceeded")
)

type tsuruBackend struct {
	msgCh      chan<- LogMessage
	quitCh     chan<- bool
	nextNotify *time.Timer
}

type wsForwarder struct {
	url           string
	token         string
	connMutex     sync.Mutex
	pingInterval  time.Duration
	pongInterval  time.Duration
	jsonEncoder   *json.Encoder
	quitCh        <-chan bool
	bufferConn    *bufferedConn
	connCreatedAt time.Time
	connMaxAge    time.Duration
	expireConnCh  chan bool
}

func (b *tsuruBackend) initialize() error {
	config.LoadConfig()
	if config.Config.TsuruEndpoint == "" {
		return fmt.Errorf("environment variable for TSURU_ENDPOINT must be set")
	}
	bufferSize := config.IntEnvOrDefault(config.DefaultBufferSize, "LOG_TSURU_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	wsPingInterval := config.SecondsEnvOrDefault(config.DefaultWsPingInterval, "LOG_TSURU_PING_INTERVAL", "LOG_WS_PING_INTERVAL")
	wsPongInterval := config.SecondsEnvOrDefault(0, "LOG_TSURU_PONG_INTERVAL", "LOG_WS_PONG_INTERVAL")
	if wsPongInterval < wsPingInterval {
		newPongInterval := wsPingInterval * 4
		bslog.Warnf("invalid WS pong interval %v (it must be higher than ping interval). Using the default value of %v", wsPongInterval, newPongInterval)
		wsPongInterval = newPongInterval
	}
	wsConnMaxAge := config.SecondsEnvOrDefault(-1, "LOG_TSURU_CONN_MAX_AGE")
	b.nextNotify = time.NewTimer(0)
	tsuruUrl, err := url.Parse(config.Config.TsuruEndpoint)
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
		token:        config.Config.TsuruToken,
		pingInterval: wsPingInterval,
		pongInterval: wsPongInterval,
		connMaxAge:   wsConnMaxAge,
	}, bufferSize)
	if err != nil {
		return err
	}
	b.msgCh = forwardChan
	b.quitCh = quitChan
	return nil
}

func (b *tsuruBackend) sendMessage(parts *rawLogParts, appName, processName, container string, tags []string) {
	msg := &app.Applog{
		Date:    parts.ts,
		AppName: appName,
		Message: string(parts.content),
		Source:  processName,
		Unit:    container,
	}
	select {
	case b.msgCh <- msg:
	default:
		select {
		case <-b.nextNotify.C:
			bslog.Errorf("Dropping log messages to tsuru due to full channel buffer.")
			b.nextNotify.Reset(time.Minute)
		default:
		}
	}
}

func (b *tsuruBackend) stop() {
	close(b.quitCh)
}

func (f *wsForwarder) initialize(quitCh <-chan bool) {
	f.quitCh = quitCh
}

func (f *wsForwarder) connect() (net.Conn, error) {
	config, err := websocket.NewConfig(f.url, "ws://localhost/")
	if err != nil {
		return nil, err
	}
	if testTlsConfig != nil {
		config.TlsConfig = testTlsConfig
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
	f.connCreatedAt = time.Now()
	f.expireConnCh = make(chan bool)
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
	f.bufferConn = newBufferedConn(ws, time.Second)
	lastPongTime := time.Now().UnixNano()
	stopWg.Add(2)
	go func() {
		defer stopWg.Done()
		defer client.Close()
		for {
			frame, err := ws.NewFrameReader()
			if err != nil {
				select {
				case <-f.expireConnCh:
					return
				default:
				}
				bslog.Errorf("[log forwarder] unable to create pong frame reader, closing websocket: %s", err)
				return
			}
			if frame.PayloadType() == websocket.PongFrame {
				atomic.StoreInt64(&lastPongTime, time.Now().UnixNano())
			}
			_, _ = io.Copy(ioutil.Discard, frame)
		}
	}()
	go func() {
		defer stopWg.Done()
		defer client.Close()
		for {
			select {
			case <-time.After(f.pingInterval):
			case <-f.quitCh:
				return
			case <-f.expireConnCh:
				return
			}
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
	f.jsonEncoder = json.NewEncoder(f.bufferConn)
	return f.bufferConn, nil
}

func (f *wsForwarder) writeWithDeadline(conn net.Conn, writer io.WriteCloser, data []byte) error {
	f.connMutex.Lock()
	defer f.connMutex.Unlock()
	f.bufferConn.mu.Lock()
	defer f.bufferConn.mu.Unlock()
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

func (f *wsForwarder) process(conn net.Conn, msg LogMessage) error {
	f.connMutex.Lock()
	defer f.connMutex.Unlock()
	err := conn.SetWriteDeadline(time.Now().Add(forwardConnWriteTimeout))
	if err != nil {
		return fmt.Errorf("error setting deadline: %s", err)
	}
	entry := msg.(*app.Applog)
	err = f.jsonEncoder.Encode(entry)
	if err != nil {
		return fmt.Errorf("error sending message: %s", err)
	}
	if time.Since(f.connCreatedAt) >= f.connMaxAge && f.connMaxAge >= 0 {
		close(f.expireConnCh)
		return errConnMaxAgeExceeded
	}
	return nil
}

func (f *wsForwarder) close(conn net.Conn) {
	f.connMutex.Lock()
	defer f.connMutex.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(forwardConnWriteTimeout)); err != nil {
		bslog.Errorf("unable to set deadline: %v", err)
	}
	conn.Close()
}

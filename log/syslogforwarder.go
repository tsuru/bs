// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"github.com/tsuru/bs/bslog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tsuru/bs/config"
)

type syslogBackend struct {
	forwardAddresses []string
	syslogLocation   *time.Location
	msgChans         []chan<- *LogMessage
	quitChans        []chan<- bool
	bufferPool       sync.Pool
	nextNotify       *time.Timer
}

type syslogForwarder struct {
	url        *url.URL
	bufferPool sync.Pool
}

func (b *syslogBackend) initialize() error {
	bufferSize := config.IntEnvOrDefault(config.DefaultBufferSize, "LOG_SYSLOG_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	forwarders := config.StringEnvOrDefault("", "LOG_SYSLOG_FORWARD_ADDRESSES", "SYSLOG_FORWARD_ADDRESSES")
	if forwarders == "" {
		return nil
	}
	forwardAddresses := strings.Split(forwarders, ",")
	syslogTimezone := config.StringEnvOrDefault("", "LOG_SYSLOG_TIMEZONE", "SYSLOG_TIMEZONE")
	b.syslogLocation = time.Local
	if syslogTimezone != "" {
		tz, err := time.LoadLocation(syslogTimezone)
		if err == nil {
			b.syslogLocation = tz
		} else {
			bslog.Warnf("unable to parse syslog timezone format: %s", err)
		}
	}
	b.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 200)
		},
	}
	b.nextNotify = time.NewTimer(0)
	for _, addr := range forwardAddresses {
		forwardUrl, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("unable to parse %q: %s", addr, err)
		}
		forwardChan, quitChan, err := processMessages(&syslogForwarder{
			url:        forwardUrl,
			bufferPool: b.bufferPool,
		}, bufferSize)
		if err != nil {
			return err
		}
		b.msgChans = append(b.msgChans, forwardChan)
		b.quitChans = append(b.quitChans, quitChan)
	}
	return nil
}

func (b *syslogBackend) sendMessage(priority int, ts time.Time, contId, appName, processName, content string) {
	lenSyslogs := len(b.msgChans)
	if lenSyslogs == 0 {
		return
	}
	buffer := b.bufferPool.Get().([]byte)[:0]
	buffer = append(buffer, '<')
	buffer = strconv.AppendInt(buffer, int64(priority), 10)
	buffer = append(buffer, '>')
	buffer = append(buffer, ts.In(b.syslogLocation).Format(time.Stamp)...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, contId...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, appName...)
	buffer = append(buffer, '[')
	buffer = append(buffer, processName...)
	buffer = append(buffer, ']', ':', ' ')
	buffer = append(buffer, content...)
	buffer = append(buffer, '\n')
	for i, ch := range b.msgChans {
		var chBuffer []byte
		if i == lenSyslogs-1 {
			chBuffer = buffer
		} else {
			chBuffer = b.bufferPool.Get().([]byte)[:0]
			chBuffer = append(chBuffer, buffer...)
		}
		msg := &LogMessage{syslogMsg: chBuffer}
		select {
		case ch <- msg:
		default:
			select {
			case <-b.nextNotify.C:
				bslog.Errorf("Dropping log messages to syslog due to full channel buffer.")
				b.nextNotify.Reset(time.Minute)
			default:
			}
		}
	}
}

func (b *syslogBackend) stop() {
	for _, ch := range b.quitChans {
		close(ch)
	}
	for _, ch := range b.msgChans {
		close(ch)
	}
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
	lenMsg := len(msg.syslogMsg)
	n, err := conn.Write(msg.syslogMsg)
	f.bufferPool.Put(msg.syslogMsg)
	if err != nil {
		return err
	}
	if n < lenMsg {
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

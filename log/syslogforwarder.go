// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/container"
)

const (
	udpMessageDefaultMTU = 1500
	udpHeaderSz          = 100 // Exagerated a bit due to possibility of ipv6 extensions, ipsec, etc.
)

type syslogBackend struct {
	syslogLocation   *time.Location
	syslogExtraStart []byte
	syslogExtraEnd   []byte
	msgChans         []chan<- LogMessage
	quitChans        []chan<- bool
	bufferPool       sync.Pool
	nextNotify       *time.Timer
}

type syslogForwarder struct {
	url           *url.URL
	bufferPool    *sync.Pool
	mtu           int
	messageLimit  int
	connCreatedAt time.Time
	connMaxAge    time.Duration
}

func (b *syslogBackend) initialize() error {
	extra := config.StringEnvOrDefault("", "LOG_SYSLOG_MESSAGE_EXTRA_START")
	if extra != "" {
		b.syslogExtraStart = []byte(os.ExpandEnv(extra) + " ")
	}
	extra = config.StringEnvOrDefault("", "LOG_SYSLOG_MESSAGE_EXTRA_END")
	if extra != "" {
		b.syslogExtraEnd = []byte(" " + os.ExpandEnv(extra))
	}
	bufferSize := config.IntEnvOrDefault(config.DefaultBufferSize, "LOG_SYSLOG_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	forwardAddresses := config.StringsEnvOrDefault(nil, "LOG_SYSLOG_FORWARD_ADDRESSES", "SYSLOG_FORWARD_ADDRESSES")
	if len(forwardAddresses) == 0 {
		return nil
	}
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
	mtu := udpMessageDefaultMTU
	mtuInterface := config.StringEnvOrDefault("eth0", "LOG_SYSLOG_MTU_NETWORK_INTERFACE")
	if mtuInterface != "" {
		iface, err := net.InterfaceByName(mtuInterface)
		if err == nil && iface.MTU > 0 {
			mtu = iface.MTU
		} else {
			bslog.Warnf("unable to read mtu from interface, using default %d: %s", mtu, err)
		}
	}
	b.bufferPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 200)
		},
	}
	b.nextNotify = time.NewTimer(0)
	connMaxAge := config.SecondsEnvOrDefault(-1, "LOG_SYSLOG_CONN_MAX_AGE")
	for _, addr := range forwardAddresses {
		forwardUrl, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("unable to parse %q: %s", addr, err)
		}
		forwardChan, quitChan, err := processMessages(&syslogForwarder{
			url:        forwardUrl,
			bufferPool: &b.bufferPool,
			mtu:        mtu,
			connMaxAge: connMaxAge,
		}, bufferSize)
		if err != nil {
			return err
		}
		b.msgChans = append(b.msgChans, forwardChan)
		b.quitChans = append(b.quitChans, quitChan)
	}
	return nil
}

type bufferWithIdx struct {
	buffer     []byte
	headerIdx  int
	contentIdx int
}

func (b *syslogBackend) sendMessage(parts *rawLogParts, c *container.Container) {
	lenSyslogs := len(b.msgChans)
	if lenSyslogs == 0 {
		return
	}
	buffer := b.bufferPool.Get().([]byte)[:0]
	buffer = append(buffer, '<')
	buffer = append(buffer, parts.priority...)
	buffer = append(buffer, '>')
	buffer = append(buffer, parts.ts.In(b.syslogLocation).Format(time.Stamp)...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, c.ShortHostname...)
	buffer = append(buffer, ' ')
	buffer = append(buffer, c.AppName...)
	buffer = append(buffer, '[')
	buffer = append(buffer, c.ProcessName...)
	buffer = append(buffer, ']', ':', ' ')
	buffer = append(buffer, b.syslogExtraStart...)
	headerIdx := len(buffer)
	buffer = append(buffer, parts.content...)
	contentIdx := len(buffer)
	buffer = append(buffer, b.syslogExtraEnd...)
	buffer = append(buffer, '\n')
	for i, ch := range b.msgChans {
		var chBuffer []byte
		if i == lenSyslogs-1 {
			chBuffer = buffer
		} else {
			chBuffer = b.bufferPool.Get().([]byte)[:0]
			chBuffer = append(chBuffer, buffer...)
		}
		select {
		case ch <- bufferWithIdx{
			buffer:     chBuffer,
			headerIdx:  headerIdx,
			contentIdx: contentIdx,
		}:
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
}

func (f *syslogForwarder) connect() (net.Conn, error) {
	conn, err := net.DialTimeout(f.url.Scheme, f.url.Host, forwardConnDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("[log forwarder] unable to connect to %q: %s", f.url, err)
	}
	if f.url.Scheme == "tcp" {
		conn = newBufferedConn(conn, time.Second)
		f.connCreatedAt = time.Now()
	} else {
		f.messageLimit = f.mtu - udpHeaderSz
	}
	return conn, nil
}

func (f *syslogForwarder) splitParts(conn net.Conn, bufIdx bufferWithIdx) error {
	fullLen := len(bufIdx.buffer)
	if f.messageLimit <= 0 || fullLen <= f.messageLimit {
		// Fast path, message fit, no manipulation needed.
		err := f.writePart(conn, bufIdx.buffer)
		f.bufferPool.Put(bufIdx.buffer) // nolint
		return err
	}
	headerBuf := bufIdx.buffer[:bufIdx.headerIdx]
	trailerBuf := bufIdx.buffer[bufIdx.contentIdx:]
	contentBuf := bufIdx.buffer[bufIdx.headerIdx:bufIdx.contentIdx]
	availableSz := f.messageLimit - (len(headerBuf) + len(trailerBuf))
	contentSz := len(contentBuf)
	nParts := contentSz / (availableSz - 6)
	if contentSz%(availableSz-6) != 0 {
		nParts++
	}
	i := 0
	for contentSz > 0 {
		var buffer []byte
		i++
		partElement := fmt.Sprintf(" (%d/%d)", i, nParts)
		sizeToUse := availableSz - len(partElement)
		if sizeToUse >= len(contentBuf) {
			sizeToUse = len(contentBuf)
			buffer = headerBuf
		} else {
			buffer = f.bufferPool.Get().([]byte)[:0]
			buffer = append(buffer, headerBuf...)
		}
		buffer = append(buffer, contentBuf[:sizeToUse]...)
		buffer = append(buffer, partElement...)
		buffer = append(buffer, trailerBuf...)
		err := f.writePart(conn, buffer)
		f.bufferPool.Put(buffer) // nolint
		if err != nil {
			return err
		}
		contentBuf = contentBuf[sizeToUse:]
		contentSz = len(contentBuf)
	}
	return nil
}

func (f *syslogForwarder) process(conn net.Conn, msg LogMessage) error {
	bufIdx := msg.(bufferWithIdx)
	err := f.splitParts(conn, bufIdx)
	if err != nil {
		return err
	}
	if f.url.Scheme == "tcp" && f.connMaxAge >= 0 && time.Since(f.connCreatedAt) >= f.connMaxAge {
		return errConnMaxAgeExceeded
	}
	return nil
}

func (f *syslogForwarder) writePart(conn net.Conn, buf []byte) error {
	err := conn.SetWriteDeadline(time.Now().Add(forwardConnWriteTimeout))
	if err != nil {
		return err
	}
	lenMsg := len(buf)
	n, err := conn.Write(buf)
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
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		bslog.Warnf("unable to reset deadline: %s", err)
	}
	conn.Close()
}

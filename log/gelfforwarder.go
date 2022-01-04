// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/container"
)

const fieldSeparators = " \t"

type gelfBackend struct {
	host             string
	chunkSize        int
	fieldsWhitelist  []string
	whitelistToField map[string]string
	msgCh            chan<- LogMessage
	quitCh           chan<- bool
	nextNotify       *time.Timer
}

func (b *gelfBackend) setup() {
	b.chunkSize = config.IntEnvOrDefault(gelf.ChunkSize, "LOG_GELF_CHUNK_SIZE")
	b.host = config.StringEnvOrDefault("localhost:12201", "LOG_GELF_HOST")
	b.fieldsWhitelist = config.StringsEnvOrDefault([]string{
		"request_id",
		"request_time",
		"request_uri",
		"status",
		"method",
		"uri",
	}, "LOG_GELF_FIELDS_WHITELIST")
	b.whitelistToField = map[string]string{}
	for _, f := range b.fieldsWhitelist {
		b.whitelistToField[f] = "_" + f
	}
	b.whitelistToField["level"] = ""
	b.nextNotify = time.NewTimer(0)
}

func (b *gelfBackend) initialize() error {
	b.setup()
	bufferSize := config.IntEnvOrDefault(config.DefaultBufferSize, "LOG_GELF_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	var err error
	b.msgCh, b.quitCh, err = processMessages(b, bufferSize)
	if err != nil {
		return err
	}
	return nil
}

func (b *gelfBackend) sendMessage(parts *rawLogParts, c *container.Container) {
	var level int32 = gelf.LOG_INFO
	if s, err := strconv.Atoi(string(parts.priority)); err == nil {
		if int32(s)&gelf.LOG_ERR == gelf.LOG_ERR {
			level = gelf.LOG_ERR
		}
	}
	msg := &gelf.Message{
		Version: "1.1",
		Host:    c.ShortHostname,
		Short:   string(parts.content),
		Level:   level,
		Extra: map[string]interface{}{
			"_app": c.AppName,
			"_pid": c.ProcessName,
		},
		RawExtra: *c.RawExtra,
		TimeUnix: float64(time.Now().UnixNano()) / float64(time.Second),
	}
	select {
	case b.msgCh <- msg:
	default:
		select {
		case <-b.nextNotify.C:
			bslog.Errorf("Dropping log messages to gelf due to full channel buffer.")
			b.nextNotify.Reset(time.Minute)
		default:
		}
	}
}
func (b *gelfBackend) stop() {
	close(b.quitCh)
}

type gelfConnWrapper struct {
	net.Conn
	*gelf.UDPWriter
}

func (w *gelfConnWrapper) Close() error {
	return w.UDPWriter.Close()
}

func (w *gelfConnWrapper) Write(msg []byte) (int, error) {
	return 0, nil
}

func (b *gelfBackend) connect() (net.Conn, error) {
	writer, err := gelf.NewUDPWriter(b.host)
	if err != nil {
		return nil, err
	}
	writer.CompressionType = gelf.CompressNone
	writer.ChunkSize = b.chunkSize
	return &gelfConnWrapper{UDPWriter: writer}, nil
}

func (b *gelfBackend) parseFields(gelfMsg *gelf.Message) {
	msg := gelfMsg.Short
	for {
		idx := strings.IndexByte(msg, '=')
		if idx == -1 {
			break
		}

		start := strings.LastIndexAny(msg[:idx], fieldSeparators)
		key := msg[start+1 : idx]
		msg = msg[idx+1:]

		underKey, allowed := b.whitelistToField[key]
		if !allowed {
			continue
		}

		end := strings.IndexAny(msg, fieldSeparators)
		if end == -1 {
			end = len(msg)
		}
		value := msg[:end]
		msg = msg[end:]

		if key == "level" {
			level := parseMsgLevel(value)
			if level > 0 {
				gelfMsg.Level = level
			}
		} else {
			gelfMsg.Extra[underKey] = value
		}
	}
}

func parseMsgLevel(level string) int32 {
	level = strings.ToUpper(level)
	switch level {
	case "EMERG", "PANIC":
		return gelf.LOG_EMERG
	case "ALERT":
		return gelf.LOG_ALERT
	case "CRIT", "CRITICAL", "FATAL":
		return gelf.LOG_CRIT
	case "ERR", "ERROR":
		return gelf.LOG_ERR
	case "WARN", "WARNING":
		return gelf.LOG_WARNING
	case "NOTICE":
		return gelf.LOG_NOTICE
	case "INFO":
		return gelf.LOG_INFO
	case "DEBUG":
		return gelf.LOG_DEBUG
	}
	return -1
}

func (b *gelfBackend) process(conn net.Conn, msg LogMessage) error {
	gelfMsg := msg.(*gelf.Message)
	b.parseFields(gelfMsg)
	return conn.(*gelfConnWrapper).WriteMessage(gelfMsg)
}

func (b *gelfBackend) close(conn net.Conn) {
	conn.Close()
}

// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

type gelfBackend struct {
	extra      json.RawMessage
	host       string
	msgCh      chan<- LogMessage
	quitCh     chan<- bool
	tryJSON    bool
	nextNotify *time.Timer
}

func (b *gelfBackend) initialize() error {
	bufferSize := config.IntEnvOrDefault(config.DefaultBufferSize, "LOG_GELF_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	b.host = config.StringEnvOrDefault("localhost:12201", "LOG_GELF_HOST")
	extra := config.StringEnvOrDefault("", "LOG_GELF_EXTRA_TAGS")
	if extra != "" {
		data := map[string]interface{}{}
		if err := json.Unmarshal([]byte(extra), &data); err != nil {
			bslog.Warnf("unable to parse gelf extra tags: %s", err)
		} else {
			b.extra = json.RawMessage(extra)
		}
	}
	b.tryJSON, _ = strconv.ParseBool(config.StringEnvOrDefault("FALSE", "LOG_GELF_TRY_JSON"))

	b.nextNotify = time.NewTimer(0)
	var err error
	b.msgCh, b.quitCh, err = processMessages(b, bufferSize)
	if err != nil {
		return err
	}
	return nil
}

func (b *gelfBackend) sendMessage(parts *rawLogParts, appName, processName, container string) {
	if len(container) > containerIDTrimSize {
		container = container[:containerIDTrimSize]
	}
	level := gelf.LOG_INFO
	if s, err := strconv.Atoi(string(parts.priority)); err == nil {
		if int32(s)&gelf.LOG_ERR == gelf.LOG_ERR {
			level = gelf.LOG_ERR
		}
	}
	msg := &gelf.Message{
		Version: "1.1",
		Host:    container,
		Short:   string(parts.content),
		Level:   level,
		Extra: map[string]interface{}{
			"_app": appName,
			"_pid": processName,
		},
		RawExtra: b.extra,
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
	*gelf.Writer
}

func (w *gelfConnWrapper) Close() error {
	return w.Writer.Close()
}

func (w *gelfConnWrapper) Write(msg []byte) (int, error) {
	return 0, nil
}

func (b *gelfBackend) connect() (net.Conn, error) {
	writer, err := gelf.NewWriter(b.host)
	if err != nil {
		return nil, err
	}
	return &gelfConnWrapper{Writer: writer}, nil
}

func (b *gelfBackend) process(conn net.Conn, msg LogMessage) error {

	var message *gelf.Message
	message = msg.(*gelf.Message)

	// Test if message has a JSON and parsing it in extra tags of GELF
	if b.tryJSON {
		first := strings.Index(message.Short, "{")
		last := strings.LastIndex(message.Short, "}") + 1
		if first >= 0 && last >= first {
			jsonMessage := message.Short[first:last]
			data := map[string]interface{}{}
			if err := json.Unmarshal([]byte(jsonMessage), &data); err == nil {
				for k, v := range data {
					if k[0] != '_' {
						message.Extra["_"+k] = v
					} else {
						message.Extra[k] = v
					}
				}
			}
		}
	}

	return conn.(*gelfConnWrapper).WriteMessage(message)
}

func (b *gelfBackend) close(conn net.Conn) {
	conn.Close()
}

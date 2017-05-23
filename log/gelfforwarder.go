// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"encoding/json"
	"strconv"

	"github.com/Graylog2/go-gelf/gelf"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

type gelfBackend struct {
	writer *gelf.Writer
	extra  json.RawMessage
}

func (b *gelfBackend) initialize() error {
	var err error
	host := config.StringEnvOrDefault("localhost:12201", "LOG_GELF_HOST")
	extra := config.StringEnvOrDefault("", "LOG_GELF_EXTRA_TAGS")
	if extra != "" {
		data := map[string]interface{}{}
		if err := json.Unmarshal([]byte(extra), &data); err != nil {
			bslog.Warnf("unable to parse gelf extra tags: %s", err)
		} else {
			b.extra = json.RawMessage(extra)
		}
	}
	b.writer, err = gelf.NewWriter(host)
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
	err := b.writer.WriteMessage(msg)
	if err != nil {
		bslog.Errorf("[log forwarder] failed to send gelf logs: %s", err)
		return
	}
}
func (b *gelfBackend) stop() {
	b.writer.Close()
}

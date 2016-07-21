// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jeromer/syslogparser"
	"github.com/jeromer/syslogparser/rfc3164"
	"github.com/jeromer/syslogparser/rfc5424"
)

type LenientFormat struct{}

func (LenientFormat) GetParser(line []byte) syslogparser.LogParser {
	return &LenientParser{line: line}
}

func (LenientFormat) GetSplitFunc() bufio.SplitFunc {
	return nil
}

type LenientParser struct {
	line      []byte
	logParts  syslogparser.LogParts
	subParser syslogparser.LogParser
}

func (p *LenientParser) Parse() error {
	groups := parseLogLine(p.line)
	if len(groups) != 7 {
		return p.defaultParsers()
	}
	priority, err := strconv.Atoi(string(groups[0]))
	if err != nil {
		return p.defaultParsers()
	}
	var ts time.Time
	if len(groups[2]) == 0 {
		ts, err = time.Parse(time.RFC3339, string(groups[1]))
		if err != nil {
			return p.defaultParsers()
		}
	} else {
		dt := string(bytes.Join(groups[1:3], []byte{' '}))
		ts, err = time.Parse(time.Stamp, dt)
		if err != nil {
			return p.defaultParsers()
		}
		ts = ts.AddDate(time.Now().Year(), 0, 0)
	}
	p.logParts = syslogparser.LogParts{
		"priority":  priority,
		"facility":  priority / 8,
		"severity":  priority % 8,
		"timestamp": ts,
		"hostname":  string(groups[3]),
		"tag":       string(groups[4]),
		"content":   string(groups[6]),
	}
	return nil
}

func (p *LenientParser) defaultParsers() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered panic parsing with %T, invalid message %q: %v", p.subParser, string(p.line), r)
		}
	}()
	p.subParser = rfc3164.NewParser(p.line)
	err = p.subParser.Parse()
	if err == nil {
		return nil
	}
	p.subParser = rfc5424.NewParser(p.line)
	return p.subParser.Parse()
}

func (p *LenientParser) Dump() syslogparser.LogParts {
	if p.subParser != nil {
		p.logParts = p.subParser.Dump()
	}
	p.logParts["rawmsg"] = p.line
	if _, ok := p.logParts["app_name"]; ok {
		p.logParts["tag"] = p.logParts["app_name"]
	}
	if _, ok := p.logParts["message"]; ok {
		p.logParts["content"] = p.logParts["message"]
	}
	if tag, ok := p.logParts["tag"].(string); ok {
		parts := strings.SplitN(tag, "/", 2)
		if len(parts) == 2 {
			p.logParts["container_id"] = parts[1]
		}
	}
	return p.logParts
}

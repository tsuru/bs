// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	"gopkg.in/mcuadros/go-syslog.v2/format"
)

type LenientFormat struct{}

func (f *LenientFormat) GetParser(line []byte) format.LogParser {
	return &LenientParser{line: line}
}

func (f *LenientFormat) GetSplitFunc() bufio.SplitFunc {
	return nil
}

type rawLogParts struct {
	ts        time.Time
	priority  []byte
	content   []byte
	container []byte
}

func (p *rawLogParts) String() string {
	return fmt.Sprintf("{log entry: %v %q %q %q}", p.ts, string(p.priority), string(p.content), string(p.container))
}

type LenientParser struct {
	line  []byte
	parts rawLogParts
}

type parseError struct {
	line []byte
	msg  string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("could not parse %q: %s", string(e.line), e.msg)
}

func (p *LenientParser) Parse() error {
	groups := parseLogLine(p.line)
	if len(groups) != 7 {
		return &parseError{line: p.line, msg: "invalid groups length"}
	}
	var err error
	if len(groups[2]) == 0 {
		p.parts.ts, err = time.Parse(time.RFC3339, string(groups[1]))
		if err != nil {
			return &parseError{line: p.line, msg: "unable to parse time as RFC3339"}
		}
	} else {
		dt := string(bytes.Join(groups[1:3], []byte{' '}))
		p.parts.ts, err = time.ParseInLocation(time.Stamp, dt, time.Local)
		if err != nil {
			return &parseError{line: p.line, msg: "unable to parse time as Stamp"}
		}
		p.parts.ts = p.parts.ts.AddDate(time.Now().Year(), 0, 0)
	}
	p.parts.priority = groups[0]
	p.parts.container = groups[4]
	idx := bytes.IndexByte(p.parts.container, '/')
	if idx != -1 {
		p.parts.container = p.parts.container[idx+1:]
	}
	p.parts.content = groups[6]
	return nil
}

func (p *LenientParser) Location(*time.Location) {
}

func (p *LenientParser) Dump() format.LogParts {
	return format.LogParts{"parts": &p.parts}
}

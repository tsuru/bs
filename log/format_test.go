// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"testing"
	"time"

	"gopkg.in/check.v1"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

func (s *S) TestLenientFormatGetParser(c *check.C) {
	lf := LenientFormat{}
	line := []byte("abc")
	parser := lf.GetParser(line)
	c.Assert(parser, check.DeepEquals, &LenientParser{line: line})
}

func (s *S) TestLenientFormatGetSplitFunc(c *check.C) {
	lf := LenientFormat{}
	splitFunc := lf.GetSplitFunc()
	c.Assert(splitFunc, check.IsNil)
}

func BenchmarkLenientParserParse(b *testing.B) {
	logLine := []byte("<30>2015-06-05T16:13:47Z vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[4843]: hey")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lp := LenientParser{line: logLine}
		_ = lp.Parse()
	}
}

func BenchmarkLenientParserParseNewFormat(b *testing.B) {
	logLine := []byte("<30> May 13 21:10:17 vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[10798]: hey")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lp := LenientParser{line: logLine}
		_ = lp.Parse()
	}
}

func BenchmarkLenientParserParseUnixFormat(b *testing.B) {
	logLine := []byte("<30>May 13 21:10:17 docker/00dfa98fe8e0[10798]: hey")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lp := LenientParser{line: logLine}
		_ = lp.Parse()
	}
}

func (s *S) TestLenientParserParse(c *check.C) {
	examples := []string{
		"<27>Jul 21 18:26:01 docker/091cafae73a9[927]: ",
		"<30>May 13 21:10:17 docker/00dfa98fe8e0[10798]: hey",
		"<30>May 13 21:10:17 docker/00dfa98fe8e0: hey",
		"<30> May 13 21:10:17 vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[10798]: ",
		"<30> May 13 21:10:17 vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[10798]: hey",
		"<30> May 13 21:10:17 vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0: hey",
		"<30>2015-06-05T16:13:47Z vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[4843]: ",
		"<30>2015-06-05T16:13:47Z vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[4843]: hey",
		"<30>2015-06-05T16:13:47Z vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0: hey",
		"<31>Dec 26 05:08:46 hostname tag/my_id[296]: ",
		"<31>Dec 26 05:08:46 hostname tag/my_id[296]: content",
	}
	expected := []format.LogParts{
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 7, 21, 18, 26, 01, 0, time.Local),
			priority:  []byte("27"),
			content:   nil,
			container: []byte("091cafae73a9"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.Local),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.Local),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.Local),
			priority:  []byte("30"),
			content:   nil,
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.Local),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.Local),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			priority:  []byte("30"),
			content:   nil,
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte("00dfa98fe8e0"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 12, 26, 5, 8, 46, 0, time.Local),
			priority:  []byte("31"),
			content:   nil,
			container: []byte("my_id"),
		}},
		{"parts": &rawLogParts{
			ts:        time.Date(time.Now().Year(), 12, 26, 5, 8, 46, 0, time.Local),
			priority:  []byte("31"),
			content:   []byte("content"),
			container: []byte("my_id"),
		}},
	}
	for i, line := range examples {
		lp := LenientParser{line: []byte(line)}
		err := lp.Parse()
		c.Assert(err, check.IsNil, check.Commentf("error in %d", i))
		parts := lp.Dump()
		c.Check(parts, check.DeepEquals, expected[i], check.Commentf("error in %d", i))
	}
}

// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"time"

	"github.com/jeromer/syslogparser"
	"gopkg.in/check.v1"
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

func (s *S) BenchmarkLenientParserParse(c *check.C) {
	logLine := []byte("<30>2015-06-05T16:13:47Z vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[4843]: hey")
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		lp := LenientParser{line: logLine}
		lp.Parse()
	}
}

func (s *S) BenchmarkLenientParserParseNewFormat(c *check.C) {
	logLine := []byte("<30> May 13 21:10:17 vagrant-ubuntu-trusty-64 docker/00dfa98fe8e0[10798]: hey")
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		lp := LenientParser{line: logLine}
		lp.Parse()
	}
}

func (s *S) BenchmarkLenientParserParseUnixFormat(c *check.C) {
	logLine := []byte("<30>May 13 21:10:17 docker/00dfa98fe8e0[10798]: hey")
	c.ResetTimer()
	for i := 0; i < c.N; i++ {
		lp := LenientParser{line: logLine}
		lp.Parse()
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
		"<165>1 2003-08-24T05:14:15.000003Z 192.0.2.1 myproc 8710 - - ",
		"<165>1 2003-08-24T05:14:15.000003Z 192.0.2.1 myproc 8710 - - content",
	}
	expected := []syslogparser.LogParts{
		{
			"priority":     27,
			"facility":     3,
			"severity":     3,
			"timestamp":    time.Date(time.Now().Year(), 7, 21, 18, 26, 01, 0, time.UTC),
			"hostname":     "",
			"tag":          "docker/091cafae73a9",
			"content":      "",
			"container_id": "091cafae73a9",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.UTC),
			"hostname":     "",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.UTC),
			"hostname":     "",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(time.Now().Year(), 5, 13, 21, 10, 17, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     30,
			"facility":     3,
			"severity":     6,
			"timestamp":    time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			"hostname":     "vagrant-ubuntu-trusty-64",
			"tag":          "docker/00dfa98fe8e0",
			"content":      "hey",
			"container_id": "00dfa98fe8e0",
		},
		{
			"priority":     31,
			"facility":     3,
			"severity":     7,
			"timestamp":    time.Date(time.Now().Year(), 12, 26, 5, 8, 46, 0, time.UTC),
			"hostname":     "hostname",
			"tag":          "tag/my_id",
			"content":      "",
			"container_id": "my_id",
		},
		{
			"priority":     31,
			"facility":     3,
			"severity":     7,
			"timestamp":    time.Date(time.Now().Year(), 12, 26, 5, 8, 46, 0, time.UTC),
			"hostname":     "hostname",
			"tag":          "tag/my_id",
			"content":      "content",
			"container_id": "my_id",
		},
		{
			"priority":        165,
			"facility":        20,
			"severity":        5,
			"timestamp":       time.Date(2003, 8, 24, 5, 14, 15, 3000, time.UTC),
			"hostname":        "192.0.2.1",
			"tag":             "myproc",
			"content":         "",
			"app_name":        "myproc",
			"proc_id":         "8710",
			"message":         "",
			"msg_id":          "-",
			"structured_data": "-",
			"version":         1,
		},
		{
			"priority":        165,
			"facility":        20,
			"severity":        5,
			"timestamp":       time.Date(2003, 8, 24, 5, 14, 15, 3000, time.UTC),
			"hostname":        "192.0.2.1",
			"tag":             "myproc",
			"content":         "content",
			"app_name":        "myproc",
			"proc_id":         "8710",
			"message":         "content",
			"msg_id":          "-",
			"structured_data": "-",
			"version":         1,
		},
	}
	for i, line := range examples {
		lp := LenientParser{line: []byte(line)}
		err := lp.Parse()
		c.Assert(err, check.IsNil, check.Commentf("error in %d", i))
		parts := lp.Dump()
		expected[i]["rawmsg"] = []byte(line)
		c.Check(parts, check.DeepEquals, expected[i], check.Commentf("error in %d", i))
	}
}

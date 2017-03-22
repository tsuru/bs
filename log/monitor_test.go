// Copyright 2017 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/check.v1"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

const (
	logEntries = `
{"log":"msg1\n","stream":"stderr","time":"2017-03-21T21:28:22.0Z"}
{"log":"msg2\n","stream":"stdout","time":"2017-03-21T21:28:32.0Z"}
{"log":"msg3\n","stream":"stderr","time":"2017-03-21T21:28:42.0Z"}
`
	additionalEntries = `
{"log":"msg4\n","stream":"stderr","time":"2017-03-21T21:28:52.0Z"}
`
)

type testHandler struct {
	parts chan format.LogParts
}

func (h *testHandler) Handle(logParts format.LogParts, _ int64, err error) {
	h.parts <- logParts
}

func withTempFile(c *check.C) string {
	f, err := ioutil.TempFile("", "bs-file-monitor")
	c.Assert(err, check.IsNil)
	_, err = f.Write([]byte(logEntries))
	c.Assert(err, check.IsNil)
	err = f.Close()
	c.Assert(err, check.IsNil)
	return f.Name()
}

func (s *S) TestFileMonitorRun(c *check.C) {
	fName := withTempFile(c)
	defer os.Remove(fName)
	th := &testHandler{parts: make(chan format.LogParts)}
	m, err := newFileMonitor(th, fName, "cont1")
	c.Assert(err, check.IsNil)
	err = m.run()
	c.Assert(err, check.IsNil)
	defer func() {
		m.stop()
		m.wait()
	}()
	ts0, _ := time.Parse(time.RFC3339, "2017-03-21T21:28:22Z")
	expectedMessages := []rawLogParts{
		{content: []byte("msg1"), ts: ts0, container: []byte("cont1"), priority: []byte("27")},
		{content: []byte("msg2"), ts: ts0.Add(10 * time.Second), container: []byte("cont1"), priority: []byte("30")},
		{content: []byte("msg3"), ts: ts0.Add(20 * time.Second), container: []byte("cont1"), priority: []byte("27")},
	}
	for _, expected := range expectedMessages {
		parts := <-th.parts
		c.Check(parts["parts"], check.DeepEquals, &expected)
	}
	c.Assert(m.alive(), check.Equals, true)
}

func (s *S) TestFileMonitorRunOnTruncate(c *check.C) {
	fName := withTempFile(c)
	defer os.Remove(fName)
	th := &testHandler{parts: make(chan format.LogParts)}
	m, err := newFileMonitor(th, fName, "cont1")
	c.Assert(err, check.IsNil)
	err = m.run()
	c.Assert(err, check.IsNil)
	defer func() {
		m.stop()
		m.wait()
	}()
	ts0, _ := time.Parse(time.RFC3339, "2017-03-21T21:28:22Z")
	expectedMessages := []rawLogParts{
		{content: []byte("msg1"), ts: ts0, container: []byte("cont1"), priority: []byte("27")},
		{content: []byte("msg2"), ts: ts0.Add(10 * time.Second), container: []byte("cont1"), priority: []byte("30")},
		{content: []byte("msg3"), ts: ts0.Add(20 * time.Second), container: []byte("cont1"), priority: []byte("27")},
	}
	for _, expected := range expectedMessages {
		parts := <-th.parts
		c.Check(parts["parts"], check.DeepEquals, &expected)
	}
	for i := 0; i < 100; i++ {
		err = ioutil.WriteFile(fName, []byte(additionalEntries), 0600)
		c.Assert(err, check.IsNil)
	}
	expectedMessages = []rawLogParts{
		{content: []byte("msg4"), ts: ts0.Add(30 * time.Second), container: []byte("cont1"), priority: []byte("27")},
	}
	for _, expected := range expectedMessages {
		parts := <-th.parts
		c.Check(parts["parts"], check.DeepEquals, &expected)
	}
	c.Assert(m.alive(), check.Equals, true)
}

func (s *S) TestFileMonitorAlive(c *check.C) {
	fName := withTempFile(c)
	defer os.Remove(fName)
	th := &testHandler{parts: make(chan format.LogParts)}
	m, err := newFileMonitor(th, fName, "cont1")
	c.Assert(err, check.IsNil)
	err = m.run()
	c.Assert(err, check.IsNil)
	defer func() {
		m.stop()
		m.wait()
	}()
	m.cmd.Process.Kill()
	for {
		if !m.alive() {
			break
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case <-time.After(5 * time.Second):
			c.Fatal("timeout waiting for not alive")
		}
	}
	c.Assert(m.alive(), check.Equals, false)
}

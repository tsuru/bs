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

func (s *S) TestLogEntryFromName(c *check.C) {
	tests := []struct {
		in  string
		out logFileEntry
	}{
		{
			in: "kube-addon-manager-minikube_kube-system_POD-cb1e80138062646f91f08090e6d5e872e83e32d227ff137e621109ac58b515f6.log",
			out: logFileEntry{
				podName:       "kube-addon-manager-minikube",
				namespace:     "kube-system",
				containerName: "POD",
				containerID:   "cb1e80138062646f91f08090e6d5e872e83e32d227ff137e621109ac58b515f6",
			},
		},
		{
			in: "kube-addon-manager-minikube_kube-system_kube-addon-manager-009fe350fb558575aa8c396f9aed216978e2c46aa9d9601d85df4c0c44eff251.log",
			out: logFileEntry{
				podName:       "kube-addon-manager-minikube",
				namespace:     "kube-system",
				containerName: "kube-addon-manager",
				containerID:   "009fe350fb558575aa8c396f9aed216978e2c46aa9d9601d85df4c0c44eff251",
			},
		},
		{
			in: "myapp-web-2453793373-cbk0k_default_POD-b166a7daa5511a7dc39a861785b00a2799bbab0b45079b0f4de78bbc537d4717.log",
			out: logFileEntry{
				podName:       "myapp-web-2453793373-cbk0k",
				namespace:     "default",
				containerName: "POD",
				containerID:   "b166a7daa5511a7dc39a861785b00a2799bbab0b45079b0f4de78bbc537d4717",
			},
		},
		{
			in: "myapp-web-2453793373-cbk0k_default_myapp-web-e50ac4567691092729a360a3a8fdc9741e81030dd3f8e90633c71cba88e32f6b.log",
			out: logFileEntry{
				podName:       "myapp-web-2453793373-cbk0k",
				namespace:     "default",
				containerName: "myapp-web",
				containerID:   "e50ac4567691092729a360a3a8fdc9741e81030dd3f8e90633c71cba88e32f6b",
			},
		},
		{
			in: "node-container-big-sibling-pool-pool3-7fz9h_default_POD-a5211d198958eb5c06cb2505a21edf50b7527fe7646b955ea9b14db65e387e2e.log",
			out: logFileEntry{
				podName:       "node-container-big-sibling-pool-pool3-7fz9h",
				namespace:     "default",
				containerName: "POD",
				containerID:   "a5211d198958eb5c06cb2505a21edf50b7527fe7646b955ea9b14db65e387e2e",
			},
		},
		{
			in: "node-container-big-sibling-pool-pool3-7fz9h_default_big-sibling-c040b6047b48cb5eacf2977bca9a40074e77f90bb6133d069cba71d44349c263.log",
			out: logFileEntry{
				podName:       "node-container-big-sibling-pool-pool3-7fz9h",
				namespace:     "default",
				containerName: "big-sibling",
				containerID:   "c040b6047b48cb5eacf2977bca9a40074e77f90bb6133d069cba71d44349c263",
			},
		},
	}
	for i, tt := range tests {
		c.Assert(logEntryFromName(tt.in), check.DeepEquals, tt.out, check.Commentf("test %d", i))
	}
}

// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tsuru/tsuru/app"
	"io"
	"io/ioutil"
	"net"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	dTesting "github.com/fsouza/go-dockerclient/testing"
	"golang.org/x/net/websocket"
	"gopkg.in/check.v1"
)

var _ = check.Suite(S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct{}

func (S) TestLogForwarderStartCachedAppName(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"udp://" + udpConn.LocalAddr().String()},
		DockerEndpoint:   "",
		AppNameEnvVar:    "",
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stop()
	lf.containerDataCache.Add("contid1", &containerData{appName: "myappname", processName: "proc1"})
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte("<30>2015-06-05T16:13:47Z myhost docker/contid1: mymsg\n")
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(buffer[:n], check.DeepEquals, []byte("<30>2015-06-05T16:13:47Z contid1 myappname[proc1]: mymsg\n"))
}

func (S) TestLogForwarderStartDockerAppName(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	lf := LogForwarder{
		BindAddress:       "udp://0.0.0.0:59317",
		ForwardAddresses:  []string{"udp://" + udpConn.LocalAddr().String()},
		DockerEndpoint:    dockerServer.URL(),
		AppNameEnvVar:     "APPNAMEVAR=",
		ProcessNameEnvVar: "PROCESSNAMEVAR=",
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stop()
	dockerClient, err := docker.NewClient(dockerServer.URL())
	c.Assert(err, check.IsNil)
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	c.Assert(err, check.IsNil)
	config := docker.Config{
		Image: "myimg",
		Cmd:   []string{"mycmd"},
		Env:   []string{"ENV1=val1", "PROCESSNAMEVAR=procx", "APPNAMEVAR=coolappname"},
	}
	opts := docker.CreateContainerOptions{Name: "myContName", Config: &config}
	cont, err := dockerClient.CreateContainer(opts)
	c.Assert(err, check.IsNil)
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", cont.ID))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	expected := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z %s coolappname[procx]: mymsg\n", cont.ID))
	c.Assert(buffer[:n], check.DeepEquals, expected)
	cached, ok := lf.containerDataCache.Get(cont.ID)
	c.Assert(ok, check.Equals, true)
	c.Assert(cached.(*containerData), check.DeepEquals, &containerData{appName: "coolappname", processName: "procx"})
}

func (S) TestLogForwarderWSForwarder(c *check.C) {
	var body bytes.Buffer
	var serverMut sync.Mutex
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		serverMut.Lock()
		defer serverMut.Unlock()
		io.Copy(&body, ws)
	}))
	defer srv.Close()
	lf := LogForwarder{
		BindAddress:   "udp://0.0.0.0:59317",
		TsuruEndpoint: srv.URL,
		TsuruToken:    "mytoken",
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	lf.containerDataCache.Add("contid1", &containerData{appName: "myappname", processName: "proc1"})
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	baseTime, err := time.Parse(time.RFC3339, "2015-06-05T16:13:47Z")
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte("<30>2015-06-05T16:13:47Z myhost docker/contid1: mymsg\n"))
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte("<30>2015-06-05T16:13:47Z myhost docker/contid1: mymsg2\n"))
	c.Assert(err, check.IsNil)
	time.Sleep(2 * time.Second)
	lf.stop()
	serverMut.Lock()
	parts := strings.Split(body.String(), "\n")
	serverMut.Unlock()
	c.Assert(parts, check.HasLen, 3)
	c.Assert(parts[2], check.Equals, "")
	var logLine app.Applog
	err = json.Unmarshal([]byte(parts[0]), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg",
		Source:  "proc1",
		AppName: "myappname",
		Unit:    "contid1",
	})
	err = json.Unmarshal([]byte(parts[1]), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg2",
		Source:  "proc1",
		AppName: "myappname",
		Unit:    "contid1",
	})
}

func (S) TestLogForwarderStartBindError(c *check.C) {
	lf := LogForwarder{
		BindAddress: "xudp://0.0.0.0:59317",
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `invalid protocol "xudp", expected tcp or udp`)
}

func (S) TestLogForwarderForwardConnError(c *check.C) {
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"xudp://127.0.0.1:1234"},
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `\[log forwarder\] unable to connect to "xudp://127.0.0.1:1234": dial xudp: unknown network xudp`)
	lf = LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"tcp://localhost:99999"},
	}
	err = lf.Start()
	c.Assert(err, check.ErrorMatches, `\[log forwarder\] unable to connect to "tcp://localhost:99999": dial tcp: invalid port 99999`)
}

func (S) BenchmarkMessagesBroadcast(c *check.C) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	startReceiver := func() net.Conn {
		addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
		c.Assert(err, check.IsNil)
		udpConn, err := net.ListenUDP("udp", addr)
		c.Assert(err, check.IsNil)
		return udpConn
	}
	forwardedConns := []net.Conn{startReceiver(), startReceiver()}
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ioutil.Discard, ws)
	}))
	defer srv.Close()
	lf := LogForwarder{
		BindAddress: "tcp://0.0.0.0:59317",
		ForwardAddresses: []string{
			"udp://" + forwardedConns[0].LocalAddr().String(),
			"udp://" + forwardedConns[1].LocalAddr().String(),
		},
		TsuruEndpoint: srv.URL,
		TsuruToken:    "mytoken",
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	lf.containerDataCache.Add("contid1", &containerData{appName: "myappname", processName: "proc1"})
	sender := func(n int) {
		conn, err := net.Dial("tcp", "127.0.0.1:59317")
		c.Assert(err, check.IsNil)
		defer conn.Close()
		msg := []byte("<30>2015-06-05T16:13:47Z myhost docker/contid1: mymsg\n")
		for i := 0; i < n; i++ {
			_, err = conn.Write(msg)
			c.Assert(err, check.IsNil)
		}
	}
	c.ResetTimer()
	goroutines := 4
	iterations := c.N
	for i := 0; i < goroutines; i++ {
		n := iterations / goroutines
		if i == 0 {
			n += iterations % goroutines
		}
		go sender(n)
	}
	for {
		val := atomic.LoadInt64(&lf.messagesCounter)
		if val == int64(iterations) {
			break
		}
		time.Sleep(10 * time.Microsecond)
	}
	lf.stop()
}

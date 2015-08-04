// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/tsuru/tsuru/app"
	"io"
	"io/ioutil"
	"net"
	"net/http"
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

var _ = check.Suite(&S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct {
	dockerServer *dTesting.DockerServer
	id           string
}

func (s *S) SetUpTest(c *check.C) {
	var err error
	s.dockerServer, err = dTesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, check.IsNil)
	dockerClient, err := docker.NewClient(s.dockerServer.URL())
	c.Assert(err, check.IsNil)
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	c.Assert(err, check.IsNil)
	config := docker.Config{
		Image: "myimg",
		Cmd:   []string{"mycmd"},
		Env:   []string{"ENV1=val1", "TSURU_PROCESSNAME=procx", "TSURU_APPNAME=coolappname"},
	}
	opts := docker.CreateContainerOptions{Name: "myContName", Config: &config}
	cont, err := dockerClient.CreateContainer(opts)
	c.Assert(err, check.IsNil)
	s.id = cont.ID
}

func (s *S) TearDownTest(c *check.C) {
	s.dockerServer.Stop()
}

func (s *S) TestLogForwarderStartCachedAppName(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"udp://" + udpConn.LocalAddr().String()},
		DockerEndpoint:   s.dockerServer.URL(),
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stop()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(buffer[:n], check.DeepEquals, []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z %s coolappname[procx]: mymsg\n", s.id)))
}

func (s *S) TestLogForwarderStartDockerAppName(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	lf := LogForwarder{
		BindAddress:      "udp://0.0.0.0:59317",
		ForwardAddresses: []string{"udp://" + udpConn.LocalAddr().String()},
		DockerEndpoint:   s.dockerServer.URL(),
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stop()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	expected := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z %s coolappname[procx]: mymsg\n", s.id))
	c.Assert(buffer[:n], check.DeepEquals, expected)
}

func (s *S) TestLogForwarderWSForwarderHTTP(c *check.C) {
	testLogForwarderWSForwarder(s, c, httptest.NewServer)
}

func (s *S) TestLogForwarderWSForwarderHTTPS(c *check.C) {
	testLogForwarderWSForwarder(s, c, httptest.NewTLSServer)
}

func testLogForwarderWSForwarder(
	s *S, c *check.C,
	serverFunc func(handler http.Handler) *httptest.Server,
) {
	var body bytes.Buffer
	var serverMut sync.Mutex
	var req *http.Request
	srv := serverFunc(websocket.Handler(func(ws *websocket.Conn) {
		serverMut.Lock()
		defer serverMut.Unlock()
		req = ws.Request()
		io.Copy(&body, ws)
	}))
	defer srv.Close()
	srvCerts := x509.NewCertPool()
	if srv.TLS != nil {
		for _, c := range srv.TLS.Certificates {
			roots, _ := x509.ParseCertificates(c.Certificate[len(c.Certificate)-1])
			for _, root := range roots {
				srvCerts.AddCert(root)
			}
		}
	}
	lf := LogForwarder{
		BindAddress:    "udp://0.0.0.0:59317",
		TsuruEndpoint:  srv.URL,
		TsuruToken:     "mytoken",
		DockerEndpoint: s.dockerServer.URL(),
		TlsConfig:      &tls.Config{RootCAs: srvCerts},
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	baseTime, err := time.Parse(time.RFC3339, "2015-06-05T16:13:47Z")
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id)))
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg2\n", s.id)))
	c.Assert(err, check.IsNil)
	time.Sleep(2 * time.Second)
	lf.stop()
	serverMut.Lock()
	parts := strings.Split(body.String(), "\n")
	c.Assert(req, check.NotNil)
	c.Assert(req.Header.Get("Authorization"), check.Equals, "bearer mytoken")
	serverMut.Unlock()
	c.Assert(parts, check.HasLen, 3)
	c.Assert(parts[2], check.Equals, "")
	var logLine app.Applog
	err = json.Unmarshal([]byte(parts[0]), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg",
		Source:  "procx",
		AppName: "coolappname",
		Unit:    s.id,
	})
	err = json.Unmarshal([]byte(parts[1]), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg2",
		Source:  "procx",
		AppName: "coolappname",
		Unit:    s.id,
	})
}

func (s *S) TestLogForwarderStartBindError(c *check.C) {
	lf := LogForwarder{
		BindAddress:    "xudp://0.0.0.0:59317",
		DockerEndpoint: s.dockerServer.URL(),
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `invalid protocol "xudp", expected tcp or udp`)
}

func (s *S) TestLogForwarderForwardConnError(c *check.C) {
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

func (s *S) BenchmarkMessagesBroadcast(c *check.C) {
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
		TsuruEndpoint:  srv.URL,
		TsuruToken:     "mytoken",
		DockerEndpoint: s.dockerServer.URL(),
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	sender := func(n int) {
		conn, err := net.Dial("tcp", "127.0.0.1:59317")
		c.Assert(err, check.IsNil)
		defer conn.Close()
		msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
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

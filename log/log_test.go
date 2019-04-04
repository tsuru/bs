// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
	docker "github.com/fsouza/go-dockerclient"
	dTesting "github.com/fsouza/go-dockerclient/testing"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/tsuru/app"
	"golang.org/x/net/websocket"
	"gopkg.in/check.v1"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

var _ = check.Suite(&S{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type S struct {
	dockerServer *dTesting.DockerServer
	id           string
	idShort      string
}

func (s *S) SetUpSuite(c *check.C) {
	var err error
	time.Local, err = time.LoadLocation("America/Fortaleza")
	c.Assert(err, check.IsNil)
}

func serverWithContainer() (*dTesting.DockerServer, string, error) {
	dockerServer, err := dTesting.NewServer("127.0.0.1:0", nil, nil)
	if err != nil {
		return nil, "", err
	}
	dockerClient, err := docker.NewClient(dockerServer.URL())
	if err != nil {
		return nil, "", err
	}
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	if err != nil {
		return nil, "", err
	}
	config := docker.Config{
		Image: "myimg",
		Cmd:   []string{"mycmd"},
		Env:   []string{"ENV1=val1", "TSURU_PROCESSNAME=procx", "TSURU_APPNAME=coolappname"},
	}
	opts := docker.CreateContainerOptions{Name: "myContName", Config: &config}
	cont, err := dockerClient.CreateContainer(opts)
	if err != nil {
		return nil, "", err
	}
	return dockerServer, cont.ID, nil
}

func addGenericContainer(name string, labels map[string]string, serverURL string) (string, error) {
	dockerClient, err := docker.NewClient(serverURL)
	if err != nil {
		return "", err
	}
	err = dockerClient.PullImage(docker.PullImageOptions{Repository: "myimg"}, docker.AuthConfiguration{})
	if err != nil {
		return "", err
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	config := docker.Config{
		Image:  "myimg",
		Cmd:    []string{"mycmd"},
		Labels: labels,
	}
	opts := docker.CreateContainerOptions{Name: name, Config: &config}
	cont, err := dockerClient.CreateContainer(opts)
	if err != nil {
		return "", err
	}
	return cont.ID, nil
}

func (s *S) SetUpTest(c *check.C) {
	var err error
	s.dockerServer, s.id, err = serverWithContainer()
	s.idShort = s.id[:12]
	c.Assert(err, check.IsNil)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "LOG_") || strings.HasPrefix(env, "TSURU_") {
			os.Unsetenv(strings.SplitN(env, "=", 2)[0])
		}
	}
}

func (s *S) TearDownTest(c *check.C) {
	s.dockerServer.Stop()
}

func (s *S) TestLogForwarderStart(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
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
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 13:13:47 %s coolappname[procx]: mymsg\n", s.idShort))
}

func (s *S) TestLogForwarderStartNoneBackend(c *check.C) {
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"none"},
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
}

func (s *S) TestLogForwarderStartWithTimezone(c *check.C) {
	os.Setenv("LOG_SYSLOG_TIMEZONE", "America/Grenada")
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
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
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 12:13:47 %s coolappname[procx]: mymsg\n", s.idShort))
}

func (s *S) TestLogForwarderWSForwarderHTTP(c *check.C) {
	testLogForwarderWSForwarder(s, c, httptest.NewServer)
}

func (s *S) TestLogForwarderWSForwarderHTTPS(c *check.C) {
	testLogForwarderWSForwarder(s, c, httptest.NewTLSServer)
}

func recvTimeout(c *check.C, ch chan string) string {
	select {
	case data := <-ch:
		return data
	case <-time.After(5 * time.Second):
		c.Fatal("timeout waiting for channel")
	}
	return ""
}

func testLogForwarderWSForwarder(
	s *S, c *check.C,
	serverFunc func(handler http.Handler) *httptest.Server,
) {
	reqCh := make(chan *http.Request, 1)
	bodyCh := make(chan string, 1)
	srv := serverFunc(websocket.Handler(func(ws *websocket.Conn) {
		reqCh <- ws.Request()
		scanner := bufio.NewScanner(ws)
		for scanner.Scan() {
			bodyCh <- scanner.Text()
		}
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
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("TSURU_TOKEN", "mytoken")
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "100")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	testTlsConfig = &tls.Config{RootCAs: srvCerts}
	lf := LogForwarder{
		EnabledBackends: []string{"tsuru"},
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	baseTime, err := time.Parse(time.RFC3339, "2015-06-05T16:13:47Z")
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id)))
	c.Assert(err, check.IsNil)
	_, err = conn.Write([]byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg2\n", s.id)))
	c.Assert(err, check.IsNil)
	select {
	case req := <-reqCh:
		c.Assert(req.Header.Get("Authorization"), check.Equals, "bearer mytoken")
	case <-time.After(5 * time.Second):
		c.Fatal("timeout waiting for request")
	}
	line1 := recvTimeout(c, bodyCh)
	line2 := recvTimeout(c, bodyCh)
	var logLine app.Applog
	err = json.Unmarshal([]byte(line1), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg",
		Source:  "procx",
		AppName: "coolappname",
		Unit:    s.idShort,
	})
	err = json.Unmarshal([]byte(line2), &logLine)
	c.Assert(err, check.IsNil)
	c.Assert(logLine, check.DeepEquals, app.Applog{
		Date:    baseTime,
		Message: "mymsg2",
		Source:  "procx",
		AppName: "coolappname",
		Unit:    s.idShort,
	})
}

func (s *S) TestLogForwarderStartBindError(c *check.C) {
	lf := LogForwarder{
		BindAddress:    "xudp://127.0.0.1:59317",
		DockerEndpoint: s.dockerServer.URL(),
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `invalid protocol "xudp", expected tcp or udp`)
}

func (s *S) TestLogForwarderStartAlreadyBound(c *check.C) {
	lf := LogForwarder{
		BindAddress:    "udp://127.0.0.1:59317",
		DockerEndpoint: s.dockerServer.URL(),
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	lf2 := LogForwarder{
		BindAddress:    "udp://127.0.0.1:59317",
		DockerEndpoint: s.dockerServer.URL(),
	}
	err = lf2.Start()
	c.Assert(err, check.ErrorMatches, `.*address already in use.*`)
}

func (s *S) TestLogForwarderForwardConnError(c *check.C) {
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "xudp://127.0.0.1:1234")
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		EnabledBackends: []string{"syslog"},
	}
	err := lf.Start()
	c.Assert(err, check.ErrorMatches, `unable to initialize log backend "syslog": \[log forwarder\] unable to connect to "xudp://127.0.0.1:1234": dial xudp: unknown network xudp`)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "tcp://localhost:99999")
	lf = LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.ErrorMatches, `unable to initialize log backend "syslog": \[log forwarder\] unable to connect to "tcp://localhost:99999":.*invalid port.*`)
}

func (s *S) TestLogForwarderOverflow(c *check.C) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	prevLog := bslog.Logger
	logBuf := bytes.NewBuffer(nil)
	bslog.Logger = log.New(logBuf, "", 0)
	defer func() {
		bslog.Logger = prevLog
	}()
	var err error
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ioutil.Discard, ws)
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "0")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	lf := LogForwarder{
		EnabledBackends: []string{"tsuru"},
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	parts := format.LogParts{"parts": &rawLogParts{
		ts:        time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
		priority:  []byte("30"),
		content:   []byte("hey"),
		container: []byte(s.id),
	}}
	wg := sync.WaitGroup{}
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				lf.Handle(parts, 0, nil)
			}
		}()
	}
	wg.Wait()
	lf.stopWait()
	c.Assert(logBuf.String(), check.Matches, `(?s).*\[ERROR\] Dropping log messages to tsuru due to full channel buffer.*`)
}

func (s *S) TestLogForwarderHandleIgnoredInvalid(c *check.C) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	prevLog := bslog.Logger
	logBuf := bytes.NewBuffer(nil)
	bslog.Logger = log.New(logBuf, "", 0)
	bslog.Debug = true
	defer func() {
		bslog.Logger = prevLog
		bslog.Debug = false
	}()
	bodyCh := make(chan string, 2)
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		data, err := ioutil.ReadAll(ws)
		c.Assert(err, check.IsNil)
		bodyCh <- string(data)
	}))
	defer srv.Close()
	os.Setenv("BS_DEBUG", "true")
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "0")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	parts := []format.LogParts{
		{"parts": &rawLogParts{
			ts:        time.Date(2015, 6, 5, 16, 13, 47, 0, time.UTC),
			priority:  []byte("30"),
			content:   []byte(""),
			container: []byte(s.id),
		}},
		{"parts": &rawLogParts{
			ts:        time.Time{},
			priority:  []byte("30"),
			content:   []byte("hey"),
			container: []byte(s.id),
		}},
	}
	expected := []func(){
		func() {
			body := recvTimeout(c, bodyCh)
			c.Assert(body, check.Equals, "")
			c.Assert(logBuf.String(), check.Not(check.Matches), `(?s).*\[log forwarder\] invalid message.*`)
		},
		func() {
			body := recvTimeout(c, bodyCh)
			c.Assert(body, check.Equals, "")
			c.Assert(logBuf.String(), check.Matches, `(?s).*\[log forwarder\] invalid message.*`)
		},
	}
	var err error
	for i, p := range parts {
		lf := LogForwarder{
			EnabledBackends: []string{"tsuru"},
			BindAddress:     "udp://127.0.0.1:59317",
			DockerEndpoint:  s.dockerServer.URL(),
		}
		err = lf.Start()
		c.Assert(err, check.IsNil)
		lf.Handle(p, 0, nil)
		lf.stopWait()
		expected[i]()
	}
}

func (s *S) TestLogForwarderTableTennis(c *check.C) {
	prevLog := bslog.Logger
	logBuf := bytes.NewBuffer(nil)
	bslog.Logger = log.New(logBuf, "", 0)
	defer func() {
		bslog.Logger = prevLog
	}()
	var err error
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ioutil.Discard, ws)
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "100")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "0.6")
	lf := LogForwarder{
		EnabledBackends: []string{"tsuru"},
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	time.Sleep(time.Second)
	lf.stopWait()
	logParts := strings.Split(logBuf.String(), "\n")
	for _, part := range logParts {
		c.Check(part, check.Not(check.Matches), `.*no pong response in.*`)
	}
}

func (s *S) TestLogForwarderTableTennisNoPong(c *check.C) {
	prevLog := bslog.Logger
	logBuf := bytes.NewBuffer(nil)
	bslog.Logger = log.New(logBuf, "", 0)
	defer func() {
		bslog.Logger = prevLog
	}()
	var err error
	done := make(chan bool)
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			frame, hErr := ws.NewFrameReader()
			if hErr == io.EOF {
				break
			}
			if frame.PayloadType() != websocket.PingFrame &&
				frame.PayloadType() != websocket.PongFrame {
				frameReader, hErr := ws.HandleFrame(frame)
				if frameReader == nil {
					continue
				}
				_, hErr = frameReader.Read(buf)
				if hErr == io.EOF {
					if trailer := frameReader.TrailerReader(); trailer != nil {
						io.Copy(ioutil.Discard, trailer)
					}
				}
			}
		}
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "100")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "0.8")
	lf := LogForwarder{
		EnabledBackends: []string{"tsuru"},
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		c.Error("timeout after 5 seconds")
	}
	lf.stopWait()
	c.Assert(logBuf.String(), check.Matches, `(?s).*no pong response in.*`)
}

func (s *S) TestLogForwarderStartWithMessageExtra(c *check.C) {
	os.Setenv("myenv", "myvalue")
	os.Setenv("LOG_SYSLOG_MESSAGE_EXTRA_START", "#val1")
	os.Setenv("LOG_SYSLOG_MESSAGE_EXTRA_END", "#val2 #${myenv}")
	defer os.Unsetenv("LOG_SYSLOG_MESSAGE_EXTRA_START")
	defer os.Unsetenv("LOG_SYSLOG_MESSAGE_EXTRA_END")
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
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
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 13:13:47 %s coolappname[procx]: #val1 mymsg #val2 #myvalue\n", s.idShort))
}

func (s *S) TestLogForwarderSyslogSplit(c *check.C) {
	os.Setenv("LOG_SYSLOG_MESSAGE_EXTRA_START", "#val1")
	os.Setenv("LOG_SYSLOG_MESSAGE_EXTRA_END", "#val2")
	os.Setenv("LOG_SYSLOG_MTU_NETWORK_INTERFACE", "invalid")
	defer os.Unsetenv("LOG_SYSLOG_MESSAGE_EXTRA_START")
	defer os.Unsetenv("LOG_SYSLOG_MESSAGE_EXTRA_END")
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	extraSz := 66
	limit := udpMessageDefaultMTU - udpHeaderSz
	maxSz := limit - extraSz - 2 - 6
	tests := []struct {
		content          string
		expectedContents []string
		expectedSizes    []int
	}{
		{
			content: "a" +
				strings.Repeat("*", maxSz) +
				"bc" +
				strings.Repeat("*", maxSz) +
				"de" +
				strings.Repeat("*", 400) +
				"f",
			expectedContents: []string{
				"a" + strings.Repeat("*", maxSz) + "b (1/3)",
				"c" + strings.Repeat("*", maxSz) + "d (2/3)",
				"e" + strings.Repeat("*", 400) + "f (3/3)",
			},
			expectedSizes: []int{limit, limit, 474},
		},
		{
			content: "a" +
				strings.Repeat("*", maxSz) +
				"bc" +
				strings.Repeat("*", maxSz) +
				"d",
			expectedContents: []string{
				"a" + strings.Repeat("*", maxSz) + "b (1/2)",
				"c" + strings.Repeat("*", maxSz) + "d (2/2)",
			},
			expectedSizes: []int{limit, limit},
		},
		{
			content: "a" +
				strings.Repeat("*", maxSz) +
				"bc" +
				strings.Repeat("*", maxSz) +
				"de",
			expectedContents: []string{
				"a" + strings.Repeat("*", maxSz) + "b (1/3)",
				"c" + strings.Repeat("*", maxSz) + "d (2/3)",
				"e (3/3)",
			},
			expectedSizes: []int{limit, limit, 73},
		},
	}
	for _, tt := range tests {
		conn, err := net.Dial("udp", "127.0.0.1:59317")
		c.Assert(err, check.IsNil)
		msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: %s\n", s.id, tt.content))
		_, err = conn.Write(msg)
		c.Assert(err, check.IsNil)
		for i, content := range tt.expectedContents {
			buffer := make([]byte, 4196)
			udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			var n int
			n, err = udpConn.Read(buffer)
			c.Assert(err, check.IsNil)
			c.Assert(n, check.Equals, tt.expectedSizes[i])
			c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 13:13:47 %s coolappname[procx]: #val1 %s #val2\n", s.idShort, content))
		}
		conn.Close()
	}
}

func (s *S) TestLogForwarderStartFromFile(c *check.C) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	dirName, err := ioutil.TempDir("", "bs-kube-log")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dirName)
	os.Setenv("LOG_KUBERNETES_LOG_DIR", dirName)
	os.Setenv("LOG_KUBERNETES_LOG_POS_DIR", dirName)
	defer os.Unsetenv("LOG_KUBERNETES_LOG_DIR")
	defer os.Unsetenv("LOG_KUBERNETES_LOG_POS_DIR")
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	name := filepath.Join(dirName, fmt.Sprintf("pod1_default_contName1-%s.log", s.id))
	err = ioutil.WriteFile(name, []byte(singleEntry), 0600)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<27>Mar 21 18:28:52 %s coolappname[procx]: msg-single\n", s.idShort))
}

func (s *S) TestLogForwarderStress(c *check.C) {
	n := 100
	done := make(chan struct{})
	data := make(chan string, n)
	tcpConn := startReceiver(n, done, data)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "tcp://"+tcpConn.Addr().String())
	lf := LogForwarder{
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err := lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("tcp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	wg := sync.WaitGroup{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg %05d\n", s.id, i))
			_, writeErr := conn.Write(msg)
			c.Assert(writeErr, check.IsNil)
		}(i)
	}
	wg.Wait()
	<-done
	close(data)
	messages := make([]string, 0, n)
	for msg := range data {
		messages = append(messages, msg)
	}
	sort.Strings(messages)
	expected := make([]string, n)
	for i := 0; i < n; i++ {
		expected[i] = fmt.Sprintf("<30>Jun  5 13:13:47 %s coolappname[procx]: mymsg %05d", s.idShort, i)
	}
	c.Assert(messages, check.DeepEquals, expected)
}

func (s *S) TestLogForwarderHandleNonTsuruApp(c *check.C) {
	contID, err := addGenericContainer("big-sibling", nil, s.dockerServer.URL())
	c.Assert(err, check.IsNil)
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	cont, err := lf.infoClient.GetContainer(contID, false, nil)
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", contID))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 13:13:47 %s big-sibling[%s]: mymsg\n", cont.ShortHostname, contID))
}

func (s *S) TestLogForwarderHandleNonTsuruAppKubernetesLabels(c *check.C) {
	contID, err := addGenericContainer("big-sibling", map[string]string{
		"io.kubernetes.pod.name":       "my-pod",
		"io.kubernetes.container.name": "my-cont",
	}, s.dockerServer.URL())
	c.Assert(err, check.IsNil)
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	c.Assert(err, check.IsNil)
	udpConn, err := net.ListenUDP("udp", addr)
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+udpConn.LocalAddr().String())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	cont, err := lf.infoClient.GetContainer(contID, false, nil)
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", contID))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)
	buffer := make([]byte, 1024)
	udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := udpConn.Read(buffer)
	c.Assert(err, check.IsNil)
	c.Assert(string(buffer[:n]), check.Equals, fmt.Sprintf("<30>Jun  5 13:13:47 %s my-cont[my-pod]: mymsg\n", cont.ShortHostname))
}

func startReceiver(expected int, ch chan struct{}, data ...chan string) net.Listener {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	tcpConn, _ := net.ListenTCP("tcp", addr)
	go func() {
		conn, err := tcpConn.Accept()
		if err != nil {
			return
		}
		counter := 0
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			counter++
			if len(data) > 0 {
				data[0] <- scanner.Text()
			}
			if counter == expected {
				close(ch)
				return
			}
		}
	}()
	return tcpConn
}

func disableLog() {
	bslog.Logger = log.New(ioutil.Discard, "", 0)
}

func BenchmarkMessagesWaitOneSyslogAddress(b *testing.B) {
	b.StopTimer()
	disableLog()
	dockerServer, contID, err := serverWithContainer()
	if err != nil {
		b.Fatal(err)
	}
	defer dockerServer.Stop()
	done := []chan struct{}{make(chan struct{})}
	forwardedConns := []net.Listener{startReceiver(b.N, done[0])}
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "tcp://"+forwardedConns[0].Addr().String())
	lf := LogForwarder{
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	rPart := &rawLogParts{
		ts:        time.Now(),
		priority:  []byte("30"),
		content:   []byte("mymsg"),
		container: []byte(contID),
	}
	parts := format.LogParts{"parts": rPart}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lf.Handle(parts, 1, nil)
	}
	close(lf.backends[0].(*syslogBackend).msgChans[0])
	<-done[0]
	b.StopTimer()
	lf.server.Kill()
	lf.Wait()
}

func BenchmarkMessagesWaitTwoSyslogAddresses(b *testing.B) {
	b.StopTimer()
	disableLog()
	dockerServer, contID, err := serverWithContainer()
	if err != nil {
		b.Fatal(err)
	}
	defer dockerServer.Stop()
	done := []chan struct{}{make(chan struct{}), make(chan struct{})}
	forwardedConns := []net.Listener{startReceiver(b.N, done[0]), startReceiver(b.N, done[1])}
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "tcp://"+forwardedConns[0].Addr().String()+",tcp://"+forwardedConns[1].Addr().String())
	lf := LogForwarder{
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  dockerServer.URL(),
		EnabledBackends: []string{"syslog"},
	}
	err = lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	rPart := &rawLogParts{
		ts:        time.Now(),
		priority:  []byte("30"),
		content:   []byte("mymsg"),
		container: []byte(contID),
	}
	parts := format.LogParts{"parts": rPart}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lf.Handle(parts, 1, nil)
	}
	close(lf.backends[0].(*syslogBackend).msgChans[0])
	close(lf.backends[0].(*syslogBackend).msgChans[1])
	<-done[0]
	<-done[1]
	b.StopTimer()
	lf.server.Kill()
	lf.Wait()
}

func BenchmarkMessagesBroadcastNonAppContainer(b *testing.B) {
	b.StopTimer()
	disableLog()
	dockerServer, _, err := serverWithContainer()
	if err != nil {
		b.Fatal(err)
	}
	defer dockerServer.Stop()
	contID, err := addGenericContainer("big-sibling", map[string]string{
		"io.kubernetes.pod.name":       "my-pod",
		"io.kubernetes.container.name": "my-cont",
	}, dockerServer.URL())
	if err != nil {
		b.Fatal(err)
	}
	startReceiver := func() net.Conn {
		addr, recErr := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		if recErr != nil {
			b.Fatal(recErr)
		}
		udpConn, recErr := net.ListenUDP("udp", addr)
		if recErr != nil {
			b.Fatal(recErr)
		}
		return udpConn
	}
	forwardedConns := []net.Conn{startReceiver(), startReceiver()}
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ioutil.Discard, ws)
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+forwardedConns[0].LocalAddr().String()+",udp://"+forwardedConns[1].LocalAddr().String())
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "1000000")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	lf := LogForwarder{
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  dockerServer.URL(),
		EnabledBackends: []string{"tsuru", "syslog"},
	}
	err = lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	defer lf.stopWait()
	rPart := &rawLogParts{
		ts:        time.Now(),
		priority:  []byte("30"),
		content:   []byte("mymsg"),
		container: []byte(contID),
	}
	parts := format.LogParts{"parts": rPart}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lf.Handle(parts, 1, nil)
	}
	b.StopTimer()
}

func BenchmarkMessagesBroadcast(b *testing.B) {
	b.StopTimer()
	disableLog()
	dockerServer, contID, err := serverWithContainer()
	if err != nil {
		b.Fatal(err)
	}
	defer dockerServer.Stop()
	startReceiver := func() net.Conn {
		addr, recErr := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		if recErr != nil {
			b.Fatal(recErr)
		}
		udpConn, recErr := net.ListenUDP("udp", addr)
		if recErr != nil {
			b.Fatal(recErr)
		}
		return udpConn
	}
	forwardedConns := []net.Conn{startReceiver(), startReceiver()}
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ioutil.Discard, ws)
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_SYSLOG_FORWARD_ADDRESSES", "udp://"+forwardedConns[0].LocalAddr().String()+",udp://"+forwardedConns[1].LocalAddr().String())
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "1000000")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	lf := LogForwarder{
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  dockerServer.URL(),
		EnabledBackends: []string{"tsuru", "syslog"},
	}
	err = lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	defer lf.stopWait()
	rPart := &rawLogParts{
		ts:        time.Now(),
		priority:  []byte("30"),
		content:   []byte("mymsg"),
		container: []byte(contID),
	}
	parts := format.LogParts{"parts": rPart}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lf.Handle(parts, 1, nil)
	}
	b.StopTimer()
}

func BenchmarkMessagesBroadcastWaitTsuru(b *testing.B) {
	b.StopTimer()
	disableLog()
	dockerServer, contID, err := serverWithContainer()
	if err != nil {
		b.Fatal(err)
	}
	defer dockerServer.Stop()
	done := make(chan bool)
	srv := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		counter := 0
		scanner := bufio.NewScanner(ws)
		for scanner.Scan() {
			counter++
			if counter == b.N {
				close(done)
				return
			}
		}
	}))
	defer srv.Close()
	os.Setenv("TSURU_ENDPOINT", srv.URL)
	os.Setenv("LOG_TSURU_BUFFER_SIZE", "1000000")
	os.Setenv("LOG_TSURU_PING_INTERVAL", "0.1")
	os.Setenv("LOG_TSURU_PONG_INTERVAL", "2.0")
	lf := LogForwarder{
		EnabledBackends: []string{"tsuru"},
		BindAddress:     "tcp://127.0.0.1:59317",
		DockerEndpoint:  dockerServer.URL(),
	}
	err = lf.Start()
	if err != nil {
		b.Fatal(err)
	}
	defer lf.stopWait()
	rPart := &rawLogParts{
		ts:        time.Now(),
		priority:  []byte("30"),
		content:   []byte("mymsg"),
		container: []byte(contID),
	}
	parts := format.LogParts{"parts": rPart}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		lf.Handle(parts, 1, nil)
	}
	close(lf.backends[0].(*tsuruBackend).msgCh)
	<-done
	b.StopTimer()
}

func (s *S) TestGelfForwarder(c *check.C) {
	defer os.Unsetenv("LOG_GELF_HOST")
	reader, err := gelf.NewReader("127.0.0.1:0")
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_GELF_HOST", reader.Addr())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"gelf"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)

	gelfMsg, err := reader.ReadMessage()
	c.Assert(err, check.IsNil)
	c.Assert(gelfMsg, check.Not(check.IsNil))

	c.Assert(gelfMsg.Version, check.Equals, "1.1")
	c.Assert(gelfMsg.Host, check.Equals, s.idShort)
	c.Assert(gelfMsg.Short, check.Equals, "mymsg")
	c.Assert(gelfMsg.Level, check.Equals, int32(gelf.LOG_INFO))
	c.Assert(gelfMsg.Extra["_app"], check.Equals, "coolappname")
	c.Assert(gelfMsg.Extra["_pid"], check.Equals, "procx")
}

func (s *S) TestGelfForwarderExtraTags(c *check.C) {
	defer os.Unsetenv("LOG_GELF_HOST")
	defer os.Unsetenv("LOG_GELF_EXTRA_TAGS")
	reader, err := gelf.NewReader("127.0.0.1:0")
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_GELF_HOST", reader.Addr())
	os.Setenv("LOG_GELF_EXTRA_TAGS", `{"_tags": "TSURU"}`)
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"gelf"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)

	gelfMsg, err := reader.ReadMessage()
	c.Assert(err, check.IsNil)
	c.Assert(gelfMsg, check.Not(check.IsNil))

	c.Assert(gelfMsg.Version, check.Equals, "1.1")
	c.Assert(gelfMsg.Host, check.Equals, s.idShort)
	c.Assert(gelfMsg.Short, check.Equals, "mymsg")
	c.Assert(gelfMsg.Level, check.Equals, int32(gelf.LOG_INFO))
	c.Assert(gelfMsg.Extra["_app"], check.Equals, "coolappname")
	c.Assert(gelfMsg.Extra["_pid"], check.Equals, "procx")
	c.Assert(gelfMsg.Extra["_tags"], check.Equals, "TSURU")
}

func (s *S) TestGelfForwarderParseExtraTags(c *check.C) {
	defer os.Unsetenv("LOG_GELF_HOST")
	defer os.Unsetenv("LOG_GELF_FIELDS_WHITELIST")
	reader, err := gelf.NewReader("127.0.0.1:0")
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_GELF_HOST", reader.Addr())
	os.Setenv("LOG_GELF_FIELDS_WHITELIST", "request_id,status,method,uri")
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"gelf"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: mymsg request_id=xdsakj invalid_field=sklsakl status=100\tmethod=get myurl.com?uri=ignored\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)

	gelfMsg, err := reader.ReadMessage()
	c.Assert(err, check.IsNil)
	c.Assert(gelfMsg, check.Not(check.IsNil))
	c.Assert(gelfMsg.Version, check.Equals, "1.1")
	c.Assert(gelfMsg.Host, check.Equals, s.idShort)
	c.Assert(gelfMsg.Short, check.Equals, "mymsg request_id=xdsakj invalid_field=sklsakl status=100\tmethod=get myurl.com?uri=ignored")
	c.Assert(gelfMsg.Level, check.Equals, int32(gelf.LOG_INFO))
	c.Assert(gelfMsg.Extra, check.DeepEquals, map[string]interface{}{
		"_app":        "coolappname",
		"_pid":        "procx",
		"_request_id": "xdsakj",
		"_status":     "100",
		"_method":     "get",
	})
}

func (s *S) TestGelfForwarderParseLevel(c *check.C) {
	defer os.Unsetenv("LOG_GELF_HOST")
	defer os.Unsetenv("LOG_GELF_FIELDS_WHITELIST")
	reader, err := gelf.NewReader("127.0.0.1:0")
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_GELF_HOST", reader.Addr())
	os.Setenv("LOG_GELF_FIELDS_WHITELIST", "request_id,status")
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"gelf"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<30>2015-06-05T16:13:47Z myhost docker/%s: level=critical mymsg\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)

	gelfMsg, err := reader.ReadMessage()
	c.Assert(err, check.IsNil)
	c.Assert(gelfMsg, check.Not(check.IsNil))
	c.Assert(gelfMsg.Version, check.Equals, "1.1")
	c.Assert(gelfMsg.Host, check.Equals, s.idShort)
	c.Assert(gelfMsg.Short, check.Equals, "level=critical mymsg")
	c.Assert(gelfMsg.Level, check.Equals, int32(gelf.LOG_CRIT))
	c.Assert(gelfMsg.Extra, check.DeepEquals, map[string]interface{}{
		"_app": "coolappname",
		"_pid": "procx",
	})
}

func (s *S) TestGelfForwarderStdErr(c *check.C) {
	defer os.Unsetenv("LOG_GELF_HOST")
	reader, err := gelf.NewReader("127.0.0.1:0")
	c.Assert(err, check.IsNil)
	os.Setenv("LOG_GELF_HOST", reader.Addr())
	lf := LogForwarder{
		BindAddress:     "udp://127.0.0.1:59317",
		DockerEndpoint:  s.dockerServer.URL(),
		EnabledBackends: []string{"gelf"},
	}
	err = lf.Start()
	c.Assert(err, check.IsNil)
	defer lf.stopWait()
	conn, err := net.Dial("udp", "127.0.0.1:59317")
	c.Assert(err, check.IsNil)
	defer conn.Close()
	msg := []byte(fmt.Sprintf("<27>2015-06-05T16:13:47Z myhost docker/%s: myerr\n", s.id))
	_, err = conn.Write(msg)
	c.Assert(err, check.IsNil)

	gelfMsg, err := reader.ReadMessage()
	c.Assert(err, check.IsNil)
	c.Assert(gelfMsg, check.Not(check.IsNil))

	c.Assert(gelfMsg.Version, check.Equals, "1.1")
	c.Assert(gelfMsg.Host, check.Equals, s.idShort)
	c.Assert(gelfMsg.Short, check.Equals, "myerr")
	c.Assert(gelfMsg.Level, check.Equals, int32(gelf.LOG_ERR))
	c.Assert(gelfMsg.Extra["_app"], check.Equals, "coolappname")
	c.Assert(gelfMsg.Extra["_pid"], check.Equals, "procx")
}

func BenchmarkMessagesGelfBackendProcess(b *testing.B) {
	b.StopTimer()
	disableLog()
	startReceiver := func() net.Conn {
		addr, recErr := net.ResolveUDPAddr("udp", "127.0.0.1:0")
		if recErr != nil {
			b.Fatal(recErr)
		}
		udpConn, recErr := net.ListenUDP("udp", addr)
		if recErr != nil {
			b.Fatal(recErr)
		}
		go func() {
			io.Copy(ioutil.Discard, udpConn)
		}()
		return udpConn
	}
	conn := startReceiver()
	os.Setenv("LOG_GELF_HOST", conn.LocalAddr().String())
	be := gelfBackend{}
	err := be.initialize()
	if err != nil {
		b.Fatal(err)
	}
	gelfConn, err := be.connect()
	if err != nil {
		b.Fatal(err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err = be.process(gelfConn, &gelf.Message{
			Version: "1.1",
			Host:    "mycont",
			Short:   "mymsg",
			Level:   gelf.LOG_ERR,
			Extra: map[string]interface{}{
				"_app": "app1",
				"_pid": "process1",
			},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}

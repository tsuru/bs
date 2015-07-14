package log

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/golang-lru"
	"github.com/jeromer/syslogparser"
	"github.com/mcuadros/go-syslog"
	"github.com/tsuru/tsuru/app"
	"golang.org/x/net/websocket"
)

const (
	forwardConnTimeout    = time.Second
	messageChanBufferSize = 1000
)

type LogMessage struct {
	logEntry  *app.Applog
	syslogMsg []byte
}

type LogForwarder struct {
	BindAddress        string
	ForwardAddresses   []string
	DockerEndpoint     string
	AppNameEnvVar      string
	ProcessNameEnvVar  string
	TsuruEndpoint      string
	TsuruToken         string
	forwardChans       []chan<- *LogMessage
	forwardQuitChans   []chan<- bool
	server             *syslog.Server
	containerDataCache *lru.Cache
	messagesCounter    int64
}

type containerData struct {
	appName     string
	processName string
}

type processable interface {
	connect() (net.Conn, error)
	process(conn net.Conn, msg *LogMessage) error
}

type syslogForwarder struct {
	url *url.URL
}

type wsForwarder struct {
	url   string
	token string
}

func processMessages(processInfo processable) (chan<- *LogMessage, chan<- bool, error) {
	ch := make(chan *LogMessage, messageChanBufferSize)
	quit := make(chan bool)
	conn, err := processInfo.connect()
	if err != nil {
		return nil, nil, err
	}
	go func() {
		var err error
		for {
			select {
			case <-quit:
				break
			default:
			}
			if conn == nil {
				conn, err = processInfo.connect()
				if err != nil {
					conn = nil
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			for msg := range ch {
				err = processInfo.process(conn, msg)
				if err != nil {
					break
				}
			}
			conn.Close()
			if err == nil {
				break
			}
			log.Printf("[log forwarder] error writing to %#v: %s", processInfo, err)
			conn = nil
		}
	}()
	return ch, quit, nil
}

func (f *syslogForwarder) connect() (net.Conn, error) {
	conn, err := net.DialTimeout(f.url.Scheme, f.url.Host, forwardConnTimeout)
	if err != nil {
		return nil, fmt.Errorf("[log forwarder] unable to connect to %q: %s", f.url, err)
	}
	return conn, nil
}

func (f *syslogForwarder) process(conn net.Conn, msg *LogMessage) error {
	n, err := conn.Write(msg.syslogMsg)
	if err != nil {
		return err
	}
	if n < len(msg.syslogMsg) {
		return fmt.Errorf("[log forwarder] short write trying to write log to %q", conn.RemoteAddr())
	}
	return nil
}

func (f *wsForwarder) connect() (net.Conn, error) {
	config, err := websocket.NewConfig(f.url, "ws://localhost/")
	if err != nil {
		return nil, err
	}
	config.Header.Add("Authorization", "bearer "+f.token)
	conn, err := websocket.DialConfig(config)
	return conn, err
}

func (f *wsForwarder) process(conn net.Conn, msg *LogMessage) error {
	data, err := json.Marshal(msg.logEntry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	n, err := conn.Write(data)
	if err != nil {
		return err
	}
	if n < len(data) {
		return fmt.Errorf("[log forwarder] short write trying to write log to %q", conn.RemoteAddr())
	}
	return nil
}

func (l *LogForwarder) initForwardConnections() error {
	for _, addr := range l.ForwardAddresses {
		forwardUrl, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("unable to parse %q: %s", addr, err)
		}
		forwardChan, quitChan, err := processMessages(&syslogForwarder{
			url: forwardUrl,
		})
		if err != nil {
			return err
		}
		l.forwardChans = append(l.forwardChans, forwardChan)
		l.forwardQuitChans = append(l.forwardQuitChans, quitChan)
	}
	return nil
}

func (l *LogForwarder) initWSConnection() error {
	if l.TsuruEndpoint == "" {
		return nil
	}
	tsuruUrl, err := url.Parse(l.TsuruEndpoint)
	if err != nil {
		return err
	}
	wsUrl := fmt.Sprintf("ws://%s/logs", tsuruUrl.Host)
	forwardChan, quitChan, err := processMessages(&wsForwarder{
		url:   wsUrl,
		token: l.TsuruToken,
	})
	if err != nil {
		return err
	}
	l.forwardChans = append(l.forwardChans, forwardChan)
	l.forwardQuitChans = append(l.forwardQuitChans, quitChan)
	return nil
}

func (l *LogForwarder) Start() (err error) {
	defer func() {
		if err != nil {
			l.stop()
		}
	}()
	err = l.initWSConnection()
	if err != nil {
		return
	}
	err = l.initForwardConnections()
	if err != nil {
		return
	}
	l.containerDataCache, err = lru.New(100)
	if err != nil {
		return
	}
	l.server = syslog.NewServer()
	l.server.SetHandler(l)
	l.server.SetFormat(LenientFormat{})
	url, err := url.Parse(l.BindAddress)
	if err != nil {
		return
	}
	if url.Scheme == "tcp" {
		err = l.server.ListenTCP(url.Host)
	} else if url.Scheme == "udp" {
		err = l.server.ListenUDP(url.Host)
	} else {
		err = fmt.Errorf("invalid protocol %q, expected tcp or udp", url.Scheme)
	}
	if err != nil {
		return
	}
	return l.server.Boot()
}

func (l *LogForwarder) stop() {
	func() {
		defer func() {
			recover()
		}()
		if l.server != nil {
			l.server.Kill()
		}
	}()
	if l.server != nil {
		l.server.Wait()
	}
	for _, ch := range l.forwardQuitChans {
		close(ch)
	}
	for _, ch := range l.forwardChans {
		close(ch)
	}
}

func (l *LogForwarder) getContainerData(containerId string) (*containerData, error) {
	if val, ok := l.containerDataCache.Get(containerId); ok {
		return val.(*containerData), nil
	}
	client, err := docker.NewClient(l.DockerEndpoint)
	if err != nil {
		return nil, err
	}
	cont, err := client.InspectContainer(containerId)
	if err != nil {
		return nil, err
	}
	var app, process string
	for _, val := range cont.Config.Env {
		if app == "" && strings.HasPrefix(val, l.AppNameEnvVar) {
			app = val[len(l.AppNameEnvVar):]
		}
		if process == "" && strings.HasPrefix(val, l.ProcessNameEnvVar) {
			process = val[len(l.ProcessNameEnvVar):]
		}
		if app != "" && process != "" {
			data := containerData{appName: app, processName: process}
			l.containerDataCache.Add(containerId, &data)
			return &data, nil
		}
	}
	return nil, fmt.Errorf("could not find app name env in %s", containerId)
}

func (l *LogForwarder) Handle(logParts syslogparser.LogParts, msgLen int64, err error) {
	contId, _ := logParts["container_id"].(string)
	if contId == "" {
		contId, _ = logParts["hostname"].(string)
	}
	contData, err := l.getContainerData(contId)
	if err != nil {
		log.Printf("[log forwarder] ignored msg %#v error to get appname: %s", logParts, err)
		return
	}
	ts, _ := logParts["timestamp"].(time.Time)
	priority, _ := logParts["priority"].(int)
	content, _ := logParts["content"].(string)
	if ts.IsZero() || priority == 0 || content == "" {
		fmt.Printf("[log forwarder] invalid message %#v", logParts)
		return
	}
	msg := &LogMessage{
		logEntry: &app.Applog{
			Date:    ts,
			AppName: contData.appName,
			Message: content,
			Source:  contData.processName,
			Unit:    contId,
		},
		syslogMsg: []byte(fmt.Sprintf("<%d>%s %s %s[%s]: %s\n",
			priority,
			ts.Format(time.RFC3339),
			contId,
			contData.appName,
			contData.processName,
			content,
		)),
	}
	for _, ch := range l.forwardChans {
		ch <- msg
	}
	atomic.AddInt64(&l.messagesCounter, int64(1))
}

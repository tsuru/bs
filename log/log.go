package log

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/golang-lru"
	"github.com/jeromer/syslogparser"
	"github.com/tsuru/tsuru/app"
	"golang.org/x/net/websocket"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type LogForwarder struct {
	BindAddress        string
	ForwardAddresses   []string
	DockerEndpoint     string
	AppNameEnvVar      string
	ProcessNameEnvVar  string
	TsuruEndpoint      string
	TsuruToken         string
	forwardChans       []chan<- []byte
	forwardQuitChans   []chan<- bool
	server             *syslog.Server
	containerDataCache *lru.Cache
	wsConn             *websocket.Conn
	wsJsonEncoder      *json.Encoder
}

type containerData struct {
	appName     string
	processName string
}

const forwardConnTimeout = time.Second

func connWriter(addr string) (chan<- []byte, chan<- bool, error) {
	ch := make(chan []byte, 100)
	quit := make(chan bool)
	forwardUrl, err := url.Parse(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse %q: %s", addr, err)
	}
	conn, err := net.DialTimeout(forwardUrl.Scheme, forwardUrl.Host, forwardConnTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to %q: %s", addr, err)
	}
	go func(conn net.Conn) {
		var err error
		for {
			select {
			case <-quit:
				break
			default:
			}
			if conn == nil {
				time.Sleep(100 * time.Millisecond)
				conn, err = net.DialTimeout(forwardUrl.Scheme, forwardUrl.Host, forwardConnTimeout)
				if err != nil {
					log.Printf("[log forwarder] unable to connect to %q: %s", addr, err)
					continue
				}
			}
			for msg := range ch {
				var n int
				n, err = conn.Write(msg)
				if err != nil {
					err = fmt.Errorf("[log forwarder] error trying to write log to %q: %s", conn.RemoteAddr(), err)
					break
				}
				if n < len(msg) {
					err = fmt.Errorf("[log forwarder] short write trying to write log to %q", conn.RemoteAddr())
					break
				}
			}
			conn.Close()
			if err == nil {
				break
			}
			log.Print(err.Error())
			conn = nil
		}
	}(conn)
	return ch, quit, nil
}

func (l *LogForwarder) initForwardConnections() error {
	l.forwardChans = make([]chan<- []byte, len(l.ForwardAddresses))
	l.forwardQuitChans = make([]chan<- bool, len(l.ForwardAddresses))
	var err error
	for i, addr := range l.ForwardAddresses {
		l.forwardChans[i], l.forwardQuitChans[i], err = connWriter(addr)
		if err != nil {
			break
		}
	}
	if err != nil {
		for _, ch := range l.forwardChans {
			if ch != nil {
				close(ch)
			}
		}
		for _, ch := range l.forwardQuitChans {
			if ch != nil {
				close(ch)
			}
		}
	}
	return err
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
	l.wsConn, err = websocket.Dial(wsUrl, "", "ws://localhost/")
	if err != nil {
		return err
	}
	l.wsJsonEncoder = json.NewEncoder(l.wsConn)
	return nil
}

func (l *LogForwarder) Start() error {
	err := l.initWSConnection()
	if err != nil {
		return err
	}
	err = l.initForwardConnections()
	if err != nil {
		return err
	}
	l.containerDataCache, err = lru.New(100)
	if err != nil {
		return err
	}
	l.server = syslog.NewServer()
	l.server.SetHandler(l)
	l.server.SetFormat(LenientFormat{})
	url, err := url.Parse(l.BindAddress)
	if err != nil {
		return err
	}
	if url.Scheme == "tcp" {
		err = l.server.ListenTCP(url.Host)
	} else if url.Scheme == "udp" {
		err = l.server.ListenUDP(url.Host)
	} else {
		return fmt.Errorf("invalid protocol %q, expected tcp or udp", url.Scheme)
	}
	if err != nil {
		return err
	}
	return l.server.Boot()
}

func (l *LogForwarder) stop() {
	func() {
		defer func() {
			recover()
		}()
		l.server.Kill()
	}()
	l.server.Wait()
	if l.wsConn != nil {
		l.wsConn.Close()
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
	tsrMessage := app.Applog{
		Date:    ts,
		AppName: contData.appName,
		Message: content,
		Source:  contData.processName,
		Unit:    contId,
	}
	for retries := 2; l.wsJsonEncoder != nil && retries > 0; retries-- {
		err = l.wsJsonEncoder.Encode(tsrMessage)
		if err == nil {
			break
		}
		log.Printf("[log forwarder] error encoding message: %s", err)
		l.initWSConnection()
	}
	if len(l.forwardChans) > 0 {
		msg := []byte(fmt.Sprintf("<%d>%s %s %s[%s]: %s\n",
			priority,
			ts.Format(time.RFC3339),
			contId,
			contData.appName,
			contData.processName,
			content,
		))
		for _, ch := range l.forwardChans {
			ch <- msg
		}
	}
}

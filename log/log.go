package log

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/golang-lru"
	"github.com/jeromer/syslogparser"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type LogForwarder struct {
	BindAddress      string
	ForwardAddresses []string
	DockerEndpoint   string
	AppNameEnvVar    string
	server           *syslog.Server
	forwardConns     []net.Conn
	appNameCache     *lru.Cache
}

func (l *LogForwarder) initForwardConnections() error {
	l.forwardConns = make([]net.Conn, len(l.ForwardAddresses))
	for i, addr := range l.ForwardAddresses {
		forwardUrl, err := url.Parse(addr)
		if err != nil {
			return fmt.Errorf("unable to parse %q: %s", addr, err)
		}
		conn, err := net.Dial(forwardUrl.Scheme, forwardUrl.Host)
		if err != nil {
			return fmt.Errorf("unable to connect to %q: %s", addr, err)
		}
		l.forwardConns[i] = conn
	}
	return nil
}

func (l *LogForwarder) Start() error {
	err := l.initForwardConnections()
	if err != nil {
		return err
	}
	l.appNameCache, err = lru.New(100)
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

func (l *LogForwarder) appName(containerId string) (string, error) {
	if val, ok := l.appNameCache.Get(containerId); ok {
		return val.(string), nil
	}
	client, err := docker.NewClient(l.DockerEndpoint)
	if err != nil {
		return "", err
	}
	cont, err := client.InspectContainer(containerId)
	if err != nil {
		return "", err
	}
	for _, val := range cont.Config.Env {
		if strings.HasPrefix(val, l.AppNameEnvVar) {
			appName := val[len(l.AppNameEnvVar):]
			l.appNameCache.Add(containerId, appName)
			return appName, nil
		}
	}
	return "", fmt.Errorf("could not find app name env in %s", containerId)
}

func (l *LogForwarder) Handle(logParts syslogparser.LogParts, msgLen int64, err error) {
	contId, _ := logParts["container_id"].(string)
	if contId == "" {
		contId, _ = logParts["hostname"].(string)
	}
	appName, err := l.appName(contId)
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
	msg := []byte(fmt.Sprintf("<%d>%s %s %s: %s\n",
		priority,
		ts.Format(time.RFC3339),
		contId,
		appName,
		content,
	))
	for _, c := range l.forwardConns {
		go func(c net.Conn) {
			n, err := c.Write(msg)
			if err != nil {
				log.Printf("[log forwarder] error trying to write log to %q: %s", c.RemoteAddr(), err)
			}
			if n < len(msg) {
				log.Printf("[log forwarder] short write trying to write log to %q", c.RemoteAddr())
			}
		}(c)
	}
}

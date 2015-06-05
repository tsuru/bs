package log

import (
	"fmt"
	"net"
	"net/url"

	"github.com/jeromer/syslogparser"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type LogForwarder struct {
	BindAddress      string
	ForwardAddresses []string
	server           *syslog.Server
	forwardConns     []net.Conn
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

func (l *LogForwarder) Handle(logParts syslogparser.LogParts, msgLen int64, err error) {
	// TODO(cezarsa): reformat message (include appname?)
	msg := logParts["rawmsg"].([]byte)
	msg = append(msg, '\n')
	for _, c := range l.forwardConns {
		n, err := c.Write(msg)
		if err != nil {
			// TODO(cezarsa): log?
		}
		if n < len(msg) {
			// TODO(cezarsa): log?
		}
	}
}

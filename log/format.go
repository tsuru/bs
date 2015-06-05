package log

import (
	"bufio"
	"regexp"
	"strconv"
	"time"

	"github.com/jeromer/syslogparser"
	"github.com/jeromer/syslogparser/rfc3164"
	"github.com/jeromer/syslogparser/rfc5424"
)

type LenientFormat struct{}

func (LenientFormat) GetParser(line []byte) syslogparser.LogParser {
	return &LenientParser{line: line}
}

func (LenientFormat) GetSplitFunc() bufio.SplitFunc {
	return nil
}

type LenientParser struct {
	line      []byte
	logParts  syslogparser.LogParts
	subParser syslogparser.LogParser
}

var goFormatRegex = regexp.MustCompile(`^<(\d+?)>\s*(.+?)\s+(.+?)\s+(.+?)(\[(\d+?)\])?:\s+(.+)$`)

func (p *LenientParser) Parse() error {
	groups := goFormatRegex.FindSubmatch(p.line)
	if len(groups) != 8 {
		return p.defaultParsers()
	}
	priority, err := strconv.Atoi(string(groups[1]))
	if err != nil {
		return p.defaultParsers()
	}
	ts, err := time.Parse(time.RFC3339, string(groups[2]))
	if err != nil {
		return p.defaultParsers()
	}
	p.logParts = syslogparser.LogParts{
		"priority":  priority,
		"facility":  priority / 8,
		"severity":  priority % 8,
		"timestamp": ts,
		"hostname":  string(groups[3]),
		"tag":       string(groups[4]),
		"proc_id":   string(groups[6]),
		"content":   string(groups[7]),
	}
	return nil
}

func (p *LenientParser) defaultParsers() error {
	p.subParser = rfc3164.NewParser(p.line)
	err := p.subParser.Parse()
	if err == nil {
		return nil
	}
	p.subParser = rfc5424.NewParser(p.line)
	return p.subParser.Parse()
}

func (p *LenientParser) Dump() syslogparser.LogParts {
	if p.subParser != nil {
		p.logParts = p.subParser.Dump()
	}
	p.logParts["rawmsg"] = p.line
	if _, ok := p.logParts["app_name"]; ok {
		p.logParts["tag"] = p.logParts["app_name"]
	}
	if _, ok := p.logParts["message"]; ok {
		p.logParts["content"] = p.logParts["message"]
	}
	return p.logParts
}

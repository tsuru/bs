// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	bsLog "github.com/tsuru/bs/log"
)

const defaultInterval = 60

var config struct {
	DockerEndpoint         string
	TsuruEndpoint          string
	TsuruToken             string
	SentinelEnvVar         string
	StatusInterval         time.Duration
	SyslogListenAddress    string
	SyslogForwardAddresses []string
}

func loadConfig() {
	config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	config.TsuruToken = os.Getenv("TSURU_TOKEN")
	config.SentinelEnvVar = os.Getenv("TSURU_SENTINEL_ENV_VAR") + "="
	statusInterval := os.Getenv("STATUS_INTERVAL")
	parsedInterval, err := strconv.Atoi(statusInterval)
	if err != nil {
		log.Printf("[WARNING] invalid interval %q. Using the default value of %d seconds", statusInterval, defaultInterval)
		parsedInterval = defaultInterval
	}
	config.StatusInterval = time.Duration(parsedInterval) * time.Second
	config.SyslogListenAddress = os.Getenv("SYSLOG_LISTEN_ADDRESS")
	config.SyslogForwardAddresses = strings.Split(os.Getenv("SYSLOG_FORWARD_ADDRESSES"), ",")
}

func statusReporter() (chan<- struct{}, <-chan struct{}) {
	abort := make(chan struct{})
	exit := make(chan struct{})
	go func(abort <-chan struct{}) {
		for {
			select {
			case <-abort:
				close(exit)
				return
			case <-time.After(config.StatusInterval):
				reportStatus()
			}
		}
	}(abort)
	return abort, exit
}

func startSignalHandler(callback func(os.Signal), signals ...os.Signal) {
	sigChan := make(chan os.Signal, 4)
	go func() {
		if signal, ok := <-sigChan; ok {
			callback(signal)
		}
	}()
	signal.Notify(sigChan, signals...)
}

func main() {
	loadConfig()
	lf := bsLog.LogForwarder{
		BindAddress:      config.SyslogListenAddress,
		ForwardAddresses: config.SyslogForwardAddresses,
	}
	err := lf.Start()
	if err != nil {
		fmt.Printf("Unable to initialize log forwarder: %s\n", err)
		os.Exit(1)
	}
	abortReporter, reporterEnded := statusReporter()
	startSignalHandler(func(signal os.Signal) {
		close(abortReporter)
	}, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	<-reporterEnded
}

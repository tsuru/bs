// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/log"
	"github.com/tsuru/bs/metric"
	"github.com/tsuru/bs/status"
)

const (
	defaultInterval       = 60
	defaultBufferSize     = 1000000
	defaultWsPingInterval = 30
	version               = "v1.1"
)

var printVersion bool

var config struct {
	DockerEndpoint         string
	TsuruEndpoint          string
	TsuruToken             string
	LogBufferSize          int
	MetricsInterval        time.Duration
	StatusInterval         time.Duration
	SyslogListenAddress    string
	SyslogForwardAddresses []string
	SyslogTimezone         string
	LogWSPingInterval      time.Duration
	LogWSPongInterval      time.Duration
}

func init() {
	flag.BoolVar(&printVersion, "version", false, "Print version and exit")
}

func loadConfig() {
	bslog.Debug, _ = strconv.ParseBool(os.Getenv("BS_DEBUG"))
	config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	config.TsuruToken = os.Getenv("TSURU_TOKEN")
	config.SyslogTimezone = os.Getenv("SYSLOG_TIMEZONE")
	statusInterval := os.Getenv("STATUS_INTERVAL")
	parsedInterval, err := strconv.Atoi(statusInterval)
	if err != nil {
		bslog.Warnf("invalid interval %q. Using the default value of %d seconds", statusInterval, defaultInterval)
		parsedInterval = defaultInterval
	}
	config.StatusInterval = time.Duration(parsedInterval) * time.Second
	metricsInterval := os.Getenv("METRICS_INTERVAL")
	parsedMetricsInterval, err := strconv.Atoi(metricsInterval)
	if err != nil {
		bslog.Warnf("invalid metrics interval %q. Using the default value of %d seconds", metricsInterval, defaultInterval)
		parsedMetricsInterval = defaultInterval
	}
	config.MetricsInterval = time.Duration(parsedMetricsInterval) * time.Second
	bufferSize := os.Getenv("LOG_BUFFER_SIZE")
	parsedBufferSize, err := strconv.Atoi(bufferSize)
	if err != nil {
		bslog.Warnf("invalid buffer size for the log. Using the default value of %d", defaultBufferSize)
		parsedBufferSize = defaultBufferSize
	}
	wsPingInterval := os.Getenv("LOG_WS_PING_INTERVAL")
	parsedWsPingInterval, err := strconv.Atoi(wsPingInterval)
	if err != nil {
		bslog.Warnf("invalid WS ping interval %q. Using the default value of %d seconds", wsPingInterval, defaultWsPingInterval)
		parsedWsPingInterval = defaultWsPingInterval
	}
	config.LogWSPingInterval = time.Duration(parsedWsPingInterval) * time.Second
	wsPongInterval := os.Getenv("LOG_WS_PONG_INTERVAL")
	parsedWsPongInterval, err := strconv.Atoi(wsPongInterval)
	if err != nil || parsedWsPongInterval < parsedWsPingInterval {
		parsedWsPongInterval = parsedWsPingInterval * 4
		bslog.Warnf("invalid WS pong interval %q (it must be higher than ping interval). Using the default value of %d seconds", wsPongInterval, parsedWsPongInterval)
	}
	config.LogWSPongInterval = time.Duration(parsedWsPongInterval) * time.Second
	config.SyslogListenAddress = os.Getenv("SYSLOG_LISTEN_ADDRESS")
	config.LogBufferSize = parsedBufferSize
	if forwarders := os.Getenv("SYSLOG_FORWARD_ADDRESSES"); forwarders != "" {
		config.SyslogForwardAddresses = strings.Split(forwarders, ",")
	} else {
		config.SyslogForwardAddresses = nil
	}
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

func onSignalDebugGoroutines(signal os.Signal) {
	var buf []byte
	var written int
	currLen := 1024
	for written == len(buf) {
		buf = make([]byte, currLen)
		written = runtime.Stack(buf, true)
		currLen *= 2
	}
	fmt.Print(string(buf[:written]))
	startSignalHandler(onSignalDebugGoroutines, syscall.SIGUSR1)
}

func onSignalDebugProfile(signal os.Signal) {
	profFileName := "memprofile.out"
	file, err := os.OpenFile(profFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		bslog.Warnf("Error trying to open profile file %q: %s", profFileName, err)
		return
	}
	err = pprof.WriteHeapProfile(file)
	if err != nil {
		bslog.Warnf("Error trying to write mem profile: %s", err)
	}
	bslog.Warnf("Wrote mem profile to %s", profFileName)
	file.Close()
	profFileName = "cpuprofile.out"
	bslog.Warnf("Starting cpu profile, writing output to %s", profFileName)
	defer bslog.Warnf("Finished cpu profile, see %s", profFileName)
	file, err = os.OpenFile(profFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		bslog.Warnf("Error trying to open profile file %q: %s", profFileName, err)
	}
	defer file.Close()
	err = pprof.StartCPUProfile(file)
	if err != nil {
		bslog.Warnf("Error trying to start cpu profile: %s", err)
	}
	defer pprof.StopCPUProfile()
	time.Sleep(30 * time.Second)
	startSignalHandler(onSignalDebugProfile, syscall.SIGUSR2)
}

func main() {
	startSignalHandler(onSignalDebugGoroutines, syscall.SIGUSR1)
	startSignalHandler(onSignalDebugProfile, syscall.SIGUSR2)
	flag.Parse()
	if printVersion {
		fmt.Printf("bs version %s\n", version)
		return
	}
	loadConfig()
	lf := log.LogForwarder{
		BufferSize:       config.LogBufferSize,
		BindAddress:      config.SyslogListenAddress,
		ForwardAddresses: config.SyslogForwardAddresses,
		DockerEndpoint:   config.DockerEndpoint,
		TsuruEndpoint:    config.TsuruEndpoint,
		TsuruToken:       config.TsuruToken,
		WSPingInterval:   config.LogWSPingInterval,
		WSPongInterval:   config.LogWSPongInterval,
		SyslogTimezone:   config.SyslogTimezone,
	}
	err := lf.Start()
	if err != nil {
		bslog.Fatalf("Unable to initialize log forwarder: %s\n", err)
	}
	mRunner := metric.NewRunner(config.DockerEndpoint, config.MetricsInterval)
	err = mRunner.Start()
	if err != nil {
		bslog.Warnf("Unable to initialize metrics runner: %s\n", err)
	}
	reporter, err := status.NewReporter(&status.ReporterConfig{
		TsuruEndpoint:  config.TsuruEndpoint,
		TsuruToken:     config.TsuruToken,
		DockerEndpoint: config.DockerEndpoint,
		Interval:       config.StatusInterval,
	})
	if err != nil {
		bslog.Fatalf("Unable to initialize status reporter: %s\n", err)
	}
	startSignalHandler(func(signal os.Signal) {
		reporter.Stop()
		mRunner.Stop()
	}, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	reporter.Wait()
}

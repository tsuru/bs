// Copyright 2016 bs authors. All rights reserved.
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
	LogSyslogBufferSize    int
	LogTsuruBufferSize     int
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

func stringEnvOrDefault(defaultValue string, envs ...string) string {
	for _, env := range envs {
		val := os.Getenv(env)
		if val != "" {
			return val
		}
	}
	if defaultValue != "" {
		bslog.Warnf("no value found for %s. Using the default value of %s", strings.Join(envs, " or "), defaultValue)
	}
	return defaultValue
}

func intEnvOrDefault(defaultValue int, envs ...string) int {
	for _, env := range envs {
		val, err := strconv.Atoi(os.Getenv(env))
		if err == nil {
			return val
		}
	}
	if defaultValue != 0 {
		bslog.Warnf("invalid value for %s. Using the default value of %d", strings.Join(envs, " or "), defaultValue)
	}
	return defaultValue
}

func secondsEnvOrDefault(defaultValue int, envs ...string) time.Duration {
	return time.Duration(intEnvOrDefault(defaultValue, envs...)) * time.Second
}

func loadConfig() {
	bslog.Debug, _ = strconv.ParseBool(os.Getenv("BS_DEBUG"))
	config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	config.TsuruToken = os.Getenv("TSURU_TOKEN")
	config.SyslogTimezone = os.Getenv("SYSLOG_TIMEZONE")
	config.SyslogListenAddress = os.Getenv("SYSLOG_LISTEN_ADDRESS")
	config.StatusInterval = secondsEnvOrDefault(defaultInterval, "STATUS_INTERVAL")
	config.MetricsInterval = secondsEnvOrDefault(defaultInterval, "METRICS_INTERVAL")
	config.LogTsuruBufferSize = intEnvOrDefault(defaultBufferSize, "LOG_TSURU_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	config.LogSyslogBufferSize = intEnvOrDefault(defaultBufferSize, "LOG_SYSLOG_BUFFER_SIZE", "LOG_BUFFER_SIZE")
	config.LogWSPingInterval = secondsEnvOrDefault(defaultWsPingInterval, "LOG_TSURU_PING_INTERVAL", "LOG_WS_PING_INTERVAL")
	config.LogWSPongInterval = secondsEnvOrDefault(0, "LOG_TSURU_PONG_INTERVAL", "LOG_WS_PONG_INTERVAL")
	if config.LogWSPongInterval < config.LogWSPingInterval {
		config.LogWSPongInterval = config.LogWSPingInterval * 4
		bslog.Warnf("invalid WS pong interval %v (it must be higher than ping interval). Using the default value of %v", config.LogWSPongInterval/4, config.LogWSPongInterval)
	}
	forwarders := stringEnvOrDefault("", "LOG_SYSLOG_FORWARD_ADDRESSES", "SYSLOG_FORWARD_ADDRESSES")
	if forwarders != "" {
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
		TsuruBufferSize:  config.LogTsuruBufferSize,
		SyslogBufferSize: config.LogSyslogBufferSize,
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

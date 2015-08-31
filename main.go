// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	bsLog "github.com/tsuru/bs/log"
	"github.com/tsuru/bs/metric"
	"github.com/tsuru/bs/status"
)

const (
	defaultInterval = 60
	version         = "v1.1"
)

var printVersion bool

var config struct {
	DockerEndpoint         string
	TsuruEndpoint          string
	TsuruToken             string
	AppNameEnvVar          string
	ProcessNameEnvVar      string
	MetricsInterval        time.Duration
	StatusInterval         time.Duration
	SyslogListenAddress    string
	SyslogForwardAddresses []string
	SyslogTimezone         string
}

func init() {
	flag.BoolVar(&printVersion, "version", false, "Print version and exit")
}

func loadConfig() {
	config.AppNameEnvVar = "TSURU_APPNAME="
	config.ProcessNameEnvVar = "TSURU_PROCESSNAME="
	config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	config.TsuruToken = os.Getenv("TSURU_TOKEN")
	config.SyslogTimezone = os.Getenv("SYSLOG_TIMEZONE")
	statusInterval := os.Getenv("STATUS_INTERVAL")
	parsedInterval, err := strconv.Atoi(statusInterval)
	if err != nil {
		log.Printf("[WARNING] invalid interval %q. Using the default value of %d seconds", statusInterval, defaultInterval)
		parsedInterval = defaultInterval
	}
	config.StatusInterval = time.Duration(parsedInterval) * time.Second
	metricsInterval := os.Getenv("METRICS_INTERVAL")
	parsedMetricsInterval, err := strconv.Atoi(metricsInterval)
	if err != nil {
		log.Printf("[WARNING] invalid metrics interval %q. Using the default value of %d seconds", metricsInterval, defaultInterval)
		parsedMetricsInterval = defaultInterval
	}
	config.MetricsInterval = time.Duration(parsedMetricsInterval) * time.Second
	config.SyslogListenAddress = os.Getenv("SYSLOG_LISTEN_ADDRESS")
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
	profFileName := "cpuprofile.out"
	log.Printf("Starting cpu profile, writing output to %s", profFileName)
	defer log.Printf("Finished cpu profile, see %s", profFileName)
	file, err := os.OpenFile(profFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0660)
	if err != nil {
		log.Printf("Error trying to open profile file %q: %s", profFileName, err)
	}
	defer file.Close()
	err = pprof.StartCPUProfile(file)
	if err != nil {
		log.Printf("Error trying to start cpu profile: %s", err)
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
	lf := bsLog.LogForwarder{
		BindAddress:      config.SyslogListenAddress,
		ForwardAddresses: config.SyslogForwardAddresses,
		DockerEndpoint:   config.DockerEndpoint,
		TsuruEndpoint:    config.TsuruEndpoint,
		TsuruToken:       config.TsuruToken,
		SyslogTimezone:   config.SyslogTimezone,
	}
	err := lf.Start()
	if err != nil {
		log.Fatalf("Unable to initialize log forwarder: %s\n", err)
	}
	mRunner := metric.NewRunner(config.DockerEndpoint, config.MetricsInterval)
	err = mRunner.Start()
	if err != nil {
		log.Printf("Unable to initialize metrics runner: %s\n", err)
	}
	reporter := status.NewReporter(&status.ReporterConfig{
		TsuruEndpoint:  config.TsuruEndpoint,
		TsuruToken:     config.TsuruToken,
		DockerEndpoint: config.DockerEndpoint,
		Interval:       config.StatusInterval,
		AppNameEnvVar:  config.AppNameEnvVar,
	})
	startSignalHandler(func(signal os.Signal) {
		reporter.Stop()
		mRunner.Stop()
	}, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	reporter.Wait()
}

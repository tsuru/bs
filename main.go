// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/gops/agent"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/log"
	"github.com/tsuru/bs/metric"
	_ "github.com/tsuru/bs/metric/logstash"
	"github.com/tsuru/bs/status"
)

const (
	version = "v1.14"
)

var printVersion bool

type StopWaiter interface {
	Stop()
	Wait()
}

func init() {
	flag.BoolVar(&printVersion, "version", false, "Print version and exit")
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
	err := agent.Listen(&agent.Options{
		NoShutdownCleanup: true,
	})
	if err != nil {
		bslog.Fatalf("Unable to initialize gops agent: %s\n", err)
	}
	defer agent.Close()
	flag.Parse()
	if printVersion {
		fmt.Printf("bs version %s\n", version)
		return
	}
	lf := log.LogForwarder{
		BindAddress:     config.Config.SyslogListenAddress,
		DockerEndpoint:  config.Config.DockerEndpoint,
		EnabledBackends: config.Config.LogBackends,
	}
	err = lf.Start()
	if err != nil {
		bslog.Fatalf("Unable to initialize log forwarder: %s\n", err)
	}
	metricsRunner, err := initializeMetricsReporter()
	if err != nil {
		bslog.Warnf("Unable to initialize metrics runner: %s\n", err)
	}
	reporter, err := status.NewReporter(&status.ReporterConfig{
		TsuruEndpoint:  config.Config.TsuruEndpoint,
		TsuruToken:     config.Config.TsuruToken,
		DockerEndpoint: config.Config.DockerEndpoint,
		Interval:       config.Config.StatusInterval,
		Kubernetes:     isKubernetes(),
	})
	if err != nil {
		bslog.Warnf("Unable to initialize status reporter: %s\n", err)
	}
	waiters := []StopWaiter{&lf}
	if metricsRunner != nil {
		waiters = append(waiters, metricsRunner)
	}
	if reporter != nil {
		waiters = append(waiters, reporter)
	}
	var signaled bool
	startSignalHandler(func(signal os.Signal) {
		signaled = true
		for _, m := range waiters {
			go m.Stop()
		}
	}, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for _, m := range waiters {
		m.Wait()
	}
	if !signaled {
		bslog.Fatalf("Exiting bs because no service could be initialized.")
	}
}

func initializeMetricsReporter() (StopWaiter, error) {
	if !config.Config.MetricsEnable {
		return nil, nil
	}
	metricsRunner := metric.NewRunner(
		config.Config.DockerEndpoint,
		config.Config.MetricsInterval,
		config.Config.MetricsBackend,
	)
	metricsRunner.EnableBasicMetrics = config.Config.MetricsEnableBasic
	metricsRunner.EnableConnMetrics = config.Config.MetricsEnableConn
	metricsRunner.EnableHostMetrics = config.Config.MetricsEnableHost
	err := metricsRunner.Start()

	if err != nil {
		return nil, err
	}

	return metricsRunner, nil
}

func isKubernetes() bool {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	return len(host) > 1 && len(port) > 1
}

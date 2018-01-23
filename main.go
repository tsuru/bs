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
	version = "v1.12-rc3"
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
	mRunner := metric.NewRunner(config.Config.DockerEndpoint, config.Config.MetricsInterval,
		config.Config.MetricsBackend)
	err = mRunner.Start()
	if err != nil {
		bslog.Warnf("Unable to initialize metrics runner: %s\n", err)
	}
	reporter, err := status.NewReporter(&status.ReporterConfig{
		TsuruEndpoint:  config.Config.TsuruEndpoint,
		TsuruToken:     config.Config.TsuruToken,
		DockerEndpoint: config.Config.DockerEndpoint,
		Interval:       config.Config.StatusInterval,
	})
	if err != nil {
		bslog.Warnf("Unable to initialize status reporter: %s\n", err)
	}
	monitorEl := []StopWaiter{&lf, mRunner}
	if reporter != nil {
		monitorEl = append(monitorEl, reporter)
	}
	var signaled bool
	startSignalHandler(func(signal os.Signal) {
		signaled = true
		for _, m := range monitorEl {
			go m.Stop()
		}
	}, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	for _, m := range monitorEl {
		m.Wait()
	}
	if !signaled {
		bslog.Fatalf("Exiting bs because no service could be initialized.")
	}
}

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
	"syscall"
	"time"

	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
	"github.com/tsuru/bs/log"
	"github.com/tsuru/bs/metric"
	_ "github.com/tsuru/bs/metric/logstash"
	"github.com/tsuru/bs/status"
)

const (
	version = "v1.10-rc2"
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
	cpufile, err := os.OpenFile("./cpuprofile.out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		bslog.Warnf("Error trying to open profile file: %s", err)
		return
	}
	memfile, err := os.OpenFile("./memprofile.out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		bslog.Warnf("Error trying to open profile file: %s", err)
		return
	}
	lockfile, err := os.OpenFile("./lockprofile.out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		bslog.Warnf("Error trying to open profile file: %s", err)
		return
	}
	bslog.Warnf("Starting profile...")
	defer bslog.Warnf("Profile done, files written: %s, %s, %s", cpufile.Name(), memfile.Name(), lockfile.Name())
	runtime.GC()
	pprof.WriteHeapProfile(memfile)
	memfile.Close()
	runtime.SetBlockProfileRate(1)
	time.Sleep(30 * time.Second)
	pprof.Lookup("block").WriteTo(lockfile, 0)
	runtime.SetBlockProfileRate(0)
	lockfile.Close()
	pprof.StartCPUProfile(cpufile)
	time.Sleep(30 * time.Second)
	pprof.StopCPUProfile()
	cpufile.Close()
	startSignalHandler(onSignalDebugProfile, syscall.SIGUSR2)
}

func main() {
	flag.Parse()
	if printVersion {
		fmt.Printf("bs version %s\n", version)
		return
	}
	startSignalHandler(onSignalDebugGoroutines, syscall.SIGUSR1)
	startSignalHandler(onSignalDebugProfile, syscall.SIGUSR2)
	lf := log.LogForwarder{
		BindAddress:     config.Config.SyslogListenAddress,
		DockerEndpoint:  config.Config.DockerEndpoint,
		EnabledBackends: config.Config.LogBackends,
	}
	err := lf.Start()
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

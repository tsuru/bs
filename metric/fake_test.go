// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

var fakeStatter fake

func init() {
	statters["fake"] = func() (statter, error) {
		return &fakeStatter, nil
	}
}

type fakeStat struct {
	container string
	image     string
	app       string
	hostname  string
	process   string
	key       string
	value     interface{}
}

type fake struct {
	stats    []fakeStat
	failures chan error
}

func (s *fake) Send(container ContainerInfo, key string, value interface{}) error {
	select {
	case err := <-s.failures:
		return err
	default:
		stat := fakeStat{
			app:       container.app,
			hostname:  container.hostname,
			process:   container.process,
			container: container.name,
			image:     container.image,
			key:       key,
			value:     value,
		}
		s.stats = append(s.stats, stat)
		return nil
	}
}

func (s *fake) SendConn(container ContainerInfo, host string) error {
	return s.Send(container, "connection", host)
}

func (s *fake) SendHost(host HostInfo, key string, value interface{}) error {
	data := ContainerInfo{app: "sysapp", process: "-", hostname: host.Name}
	return s.Send(data, key, value)
}

func (s *fake) prepareFailure(err error) {
	s.failures <- err
}

func (s *fake) reset() {
	s.failures = make(chan error, 4)
	s.stats = nil
}

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
	app      string
	hostname string
	process  string
	key      string
	value    string
}

type fake struct {
	stats    []fakeStat
	failures chan error
}

func (s *fake) Send(app, hostname, process, key, value string) error {
	select {
	case err := <-s.failures:
		return err
	default:
		stat := fakeStat{
			app:      app,
			hostname: hostname,
			process:  process,
			key:      key,
			value:    value,
		}
		s.stats = append(s.stats, stat)
		return nil
	}
}

func (s *fake) prepareFailure(err error) {
	s.failures <- err
}

func (s *fake) reset() {
	s.failures = make(chan error, 4)
	s.stats = nil
}

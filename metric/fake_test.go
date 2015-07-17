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

type fake struct{}

func (s *fake) Send(app, hostname, process, key, value string) error {
	return nil
}

// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"gopkg.in/check.v1"
)

func (s *S) TestRegister(c *check.C) {
	var b Backend
	backendCreator := func() (Backend, error) {
		return b, nil
	}
	Register("mybackend", backendCreator)
}

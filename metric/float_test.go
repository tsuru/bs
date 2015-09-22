// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"encoding/json"

	"gopkg.in/check.v1"
)

func (s *S) TestFloatMarshal(c *check.C) {
	m := []interface{}{float(1), float(1.5), float(10.4823748)}
	expected := `[1.0,1.5,10.4823748]`
	got, err := json.Marshal(m)
	c.Assert(err, check.IsNil)
	c.Assert(string(got), check.Equals, expected)
}

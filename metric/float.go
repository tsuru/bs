// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"strconv"
	"strings"
)

type float float64

func (f float) MarshalJSON() ([]byte, error) {
	formatted := strconv.FormatFloat(float64(f), 'f', -1, 64)
	if !strings.Contains(formatted, ".") {
		formatted += ".0"
	}
	return []byte(formatted), nil
}

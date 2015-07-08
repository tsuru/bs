// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import "strings"

type statter interface {
	Send(key, value string) error
}

func getStatter(container *container) statter {
	statters := map[string]statter{
		"statsd":   &statsd{},
		"logstash": &logStash{},
	}
	if container.Config != nil {
		for _, val := range container.Config.Env {
			if strings.HasPrefix(val, "TSURU_METRICS_BACKEND") {
				statterName := strings.Replace(val, "TSURU_METRICS_BACKEND=", "", -1)
				st, ok := statters[statterName]
				if ok {
					return st
				}
			}
		}
	}
	return &fake{}
}

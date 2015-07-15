// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
	"net/http"
	"net/http/httptest"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

func (S) TestMetricsEnabled(c *check.C) {
	var cont container
	config := docker.Config{}
	cont.Config = &config
	enabled := cont.metricEnabled()
	c.Assert(enabled, check.Equals, false)
	config = docker.Config{
		Env: []string{"TSURU_METRICS_BACKEND=logstash"},
	}
	cont.Config = &config
	enabled = cont.metricEnabled()
	c.Assert(enabled, check.Equals, true)
}

func (S) TestContainerMetric(c *check.C) {
	jsonStats := `{
       "read" : "2015-01-08T22:57:31.547920715Z",
       "network" : {
          "rx_dropped" : 0,
          "rx_bytes" : 648,
          "rx_errors" : 0,
          "tx_packets" : 8,
          "tx_dropped" : 0,
          "rx_packets" : 8,
          "tx_errors" : 0,
          "tx_bytes" : 648
       },
       "memory_stats" : {
          "stats" : {
             "total_pgmajfault" : 0,
             "cache" : 0,
             "mapped_file" : 0,
             "total_inactive_file" : 0,
             "pgpgout" : 414,
             "rss" : 6537216,
             "total_mapped_file" : 0,
             "writeback" : 0,
             "unevictable" : 0,
             "pgpgin" : 477,
             "total_unevictable" : 0,
             "pgmajfault" : 0,
             "total_rss" : 6537216,
             "total_rss_huge" : 6291456,
             "total_writeback" : 0,
             "total_inactive_anon" : 0,
             "rss_huge" : 6291456,
         "hierarchical_memory_limit": 189204833,
             "total_pgfault" : 964,
             "total_active_file" : 0,
             "active_anon" : 6537216,
             "total_active_anon" : 6537216,
             "total_pgpgout" : 414,
             "total_cache" : 0,
             "inactive_anon" : 0,
             "active_file" : 0,
             "pgfault" : 964,
             "inactive_file" : 0,
             "total_pgpgin" : 477
          },
          "max_usage" : 6651904,
          "usage" : 6537216,
          "failcnt" : 0,
          "limit" : 67108864
       },
       "blkio_stats": {
          "io_service_bytes_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 428795731968
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 388177920
             }
          ],
          "io_serviced_recursive": [
             {
                "major": 8,
                "minor": 0,
                "op": "Read",
                "value": 25994442
             },
             {
                "major": 8,
                "minor": 0,
                "op": "Write",
                "value": 1734
             }
          ],
          "io_queue_recursive": [],
          "io_service_time_recursive": [],
          "io_wait_time_recursive": [],
          "io_merged_recursive": [],
          "io_time_recursive": [],
          "sectors_recursive": []
       },
       "cpu_stats" : {
          "cpu_usage" : {
             "percpu_usage" : [
                16970827,
                1839451,
                7107380,
                10571290
             ],
             "usage_in_usermode" : 10000000,
             "total_usage" : 36488948,
             "usage_in_kernelmode" : 20000000
          },
          "system_cpu_usage" : 20091722000000000
       }
    }`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(jsonStats))
	}))
	defer server.Close()
	var cont container
	stats, err := cont.metrics(server.URL)
	expected := map[string]string{
		"mem_pct_max": "9.74",
		"cpu_max":     "0.00",
		"mem_max":     "6537216",
	}
	c.Assert(stats, check.DeepEquals, expected)
	c.Assert(err, check.IsNil)
}

func (s S) TestGetContainer(c *check.C) {
	bogusContainers := []bogusContainer{
		{config: docker.Config{Image: "tsuru/python", Env: []string{"HOME=/", "TSURU_APPNAME=someapp"}}, state: docker.State{Running: true}},
	}
	dockerServer, containers := s.startDockerServer(bogusContainers, nil, c)
	defer dockerServer.Stop()
	cont, err := getContainer(dockerServer.URL(), containers[0].ID)
	c.Assert(err, check.IsNil)
	c.Assert(cont.ID, check.Equals, containers[0].ID)
}

func (S) TestContainerAppName(c *check.C) {
	var cont container
	config := docker.Config{}
	cont.Config = &config
	appName := cont.appName()
	c.Assert(appName, check.Equals, "")
	config = docker.Config{
		Env: []string{"TSURU_APPNAME=appz"},
	}
	cont.Config = &config
	appName = cont.appName()
	c.Assert(appName, check.Equals, "appz")
}

func (S) TestContainerProcess(c *check.C) {
	var cont container
	config := docker.Config{}
	cont.Config = &config
	process := cont.process()
	c.Assert(process, check.Equals, "")
	config = docker.Config{
		Env: []string{"TSURU_PROCESSNAME=web"},
	}
	cont.Config = &config
	process = cont.process()
	c.Assert(process, check.Equals, "web")
}

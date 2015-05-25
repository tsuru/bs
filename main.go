// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
)

var config struct {
	DockerEndpoint string
	TsuruEndpoint  string
	TsuruToken     string
	SentinelEnvVar string
}

func loadConfig() {
	config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	config.TsuruToken = os.Getenv("TSURU_TOKEN")
	config.SentinelEnvVar = os.Getenv("TSURU_SENTINEL_ENV_VAR") + "="
}

func main() {
}

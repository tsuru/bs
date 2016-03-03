// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/tsuru/bs/bslog"
)

const (
	DefaultInterval       = 60
	DefaultBufferSize     = 1000000
	DefaultWsPingInterval = 30
)

var Config struct {
	DockerEndpoint      string
	TsuruEndpoint       string
	TsuruToken          string
	MetricsInterval     time.Duration
	StatusInterval      time.Duration
	SyslogListenAddress string
	LogBackends         []string
}

func init() {
	LoadConfig()
}

func LoadConfig() {
	bslog.Debug, _ = strconv.ParseBool(os.Getenv("BS_DEBUG"))
	Config.DockerEndpoint = os.Getenv("DOCKER_ENDPOINT")
	Config.TsuruEndpoint = os.Getenv("TSURU_ENDPOINT")
	Config.TsuruToken = os.Getenv("TSURU_TOKEN")
	Config.SyslogListenAddress = os.Getenv("SYSLOG_LISTEN_ADDRESS")
	Config.StatusInterval = SecondsEnvOrDefault(DefaultInterval, "STATUS_INTERVAL")
	Config.MetricsInterval = SecondsEnvOrDefault(DefaultInterval, "METRICS_INTERVAL")
	Config.LogBackends = StringsEnvOrDefault([]string{"tsuru", "syslog"}, "LOG_BACKENDS")
}

func envOrDefault(convert func(string) interface{}, defaultValue interface{}, envs ...string) interface{} {
	for i, env := range envs {
		val := os.Getenv(env)
		converted := convert(val)
		if converted != nil {
			if i > 0 {
				bslog.Warnf("The environment variable %s is deprecated. Please set %s in the future.", env, envs[0])
			}
			return converted
		}
	}
	if defaultValue != nil && !reflect.DeepEqual(defaultValue, reflect.Zero(reflect.ValueOf(defaultValue).Type()).Interface()) {
		bslog.Warnf("invalid value for %s. Using the default value of %v", strings.Join(envs, " or "), defaultValue)
	}
	return defaultValue
}

func StringEnvOrDefault(defaultValue string, envs ...string) string {
	return envOrDefault(func(v string) interface{} {
		if v == "" {
			return nil
		}
		return v
	}, defaultValue, envs...).(string)
}

func StringsEnvOrDefault(defaultValue []string, envs ...string) []string {
	value := envOrDefault(func(v string) interface{} {
		if v == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}, defaultValue, envs...)
	if value != nil {
		return value.([]string)
	}
	return nil
}

func IntEnvOrDefault(defaultValue int, envs ...string) int {
	return envOrDefault(func(v string) interface{} {
		val, err := strconv.Atoi(v)
		if err != nil {
			return nil
		}
		return val
	}, defaultValue, envs...).(int)
}

func SecondsEnvOrDefault(defaultValue float64, envs ...string) time.Duration {
	return time.Duration(envOrDefault(func(v string) interface{} {
		val, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return val
	}, defaultValue, envs...).(float64) * float64(time.Second))
}

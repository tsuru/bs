// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package container

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/tsuru/bs/config"

	docker "github.com/fsouza/go-dockerclient"
	lru "github.com/hashicorp/golang-lru"
	"github.com/tsuru/bs/bslog"
	"github.com/tsuru/bs/config"
)

var (
	ErrTsuruVariablesNotFound = errors.New("could not find wanted envs")

	hexRegex = regexp.MustCompile(`(?i)^[a-f0-9]+$`)

	appNameLabels     = []string{"bs.tsuru.io/log-app-name", "log-app-name", "io.kubernetes.container.name"}
	processNameLabels = []string{"bs.tsuru.io/log-process-name", "log-process-name", "io.kubernetes.pod.name"}
	logTagLabels      = []string{"bs.tsuru.io/log-tags", "log-tags"}
	labelIsIsolated   = []string{"is-isolated-run", "tsuru.io/is-isolated-run"}
)

const containerIDTrimSize = 12

type InfoClient struct {
	dockerInfo     *config.DockerConfig
	client         *docker.Client
	containerCache *lru.Cache

	extra        json.RawMessage
	decodedExtra map[string]string
}

type Container struct {
	docker.Container
	client        *InfoClient
	TsuruApp      bool
	AppName       string
	ProcessName   string
	ShortHostname string

	Tags     []string
	RawExtra *json.RawMessage
}

const (
	fullTimeout = 1 * time.Minute
)

func NewClient(dockerInfo *config.DockerConfig) (*InfoClient, error) {
	c := InfoClient{dockerInfo: dockerInfo}
	var err error
	c.containerCache, err = lru.New(100)
	if err != nil {
		return nil, err
	}
	if dockerInfo.UseTLS {
		c.client, err = docker.NewTLSClient(dockerInfo.Endpoint, dockerInfo.CertFile,
			dockerInfo.KeyFile, dockerInfo.CaFile)
	} else {
		c.client, err = docker.NewClient(dockerInfo.Endpoint)
	}
	if err != nil {
		return nil, err
	}
	c.client.SetTimeout(fullTimeout)
	c.configGelfExtraTags()
	return &c, nil
}

func (c *InfoClient) GetClient() *docker.Client {
	return c.client
}

func (c *InfoClient) ListContainers() ([]docker.APIContainers, error) {
	return c.client.ListContainers(docker.ListContainersOptions{})
}

// GetContainer returns the container with the provided id if the container has the required
// environment variable. It may use a cache to prevent calling the docker api.
func (c *InfoClient) GetContainer(containerId string, useCache bool, requiredEnvs []string) (*Container, error) {
	cont, err := c.getContainer(containerId, useCache)
	if err != nil {
		return nil, err
	}
	if len(requiredEnvs) > 0 {
		if cont.HasEnvs(requiredEnvs) {
			return cont, nil
		}
		return nil, ErrTsuruVariablesNotFound
	}
	return cont, nil
}

// GetAppContainer returns the container with id containerId if that container
// is an tsuru application. It may use a cache to prevent calling the docker api.
func (c *InfoClient) GetAppContainer(containerId string, useCache bool) (*Container, error) {
	return c.GetContainer(containerId, useCache, []string{"TSURU_APPNAME"})
}

func (c *InfoClient) getContainer(containerId string, useCache bool) (*Container, error) {
	if useCache {
		if val, ok := c.containerCache.Get(containerId); ok {
			return val.(*Container), nil
		}
	}

	cont, err := c.client.InspectContainer(containerId)
	if err != nil {
		return nil, err
	}

	contData := Container{Container: *cont, client: c, Tags: []string{}}

	toFill := map[string]*string{
		"TSURU_APPNAME=":     &contData.AppName,
		"TSURU_PROCESSNAME=": &contData.ProcessName,
	}
	for k, v := range toFill {
		for _, env := range cont.Config.Env {
			if strings.HasPrefix(env, k) {
				*v = env[len(k):]
			}
		}
	}

	if contData.AppName == "" {
		name, ok := contData.GetLabelAny(appNameLabels...)
		if !ok {
			name = strings.TrimPrefix(contData.Name, "/")
		}
		process, ok := contData.GetLabelAny(processNameLabels...)
		if !ok {
			process = contData.ID
		}
		contData.AppName = name
		contData.ProcessName = process
	} else {
		contData.TsuruApp = true
	}

	if tags, ok := contData.GetLabelAny(logTagLabels...); ok {
		contData.Tags = pruneTags(strings.Split(tags, ","))
	}
	contData.RawExtra = c.genRawExtra(contData.Tags)

	contData.ShortHostname = contData.Config.Hostname
	if hexRegex.MatchString(contData.Config.Hostname) && len(contData.Config.Hostname) > containerIDTrimSize {
		contData.ShortHostname = contData.Config.Hostname[:containerIDTrimSize]
	}

	c.containerCache.Add(containerId, &contData)
	return &contData, nil
}

func (c *InfoClient) genRawExtra(newTags []string) *json.RawMessage {
	extra := &c.extra
	if _tags, ok := c.decodedExtra["_tags"]; ok && len(newTags) > 0 {
		envTags := pruneTags(strings.Split(_tags, ","))
		tags := append(envTags, newTags...)
		newExtra, err := newExtraWithTags(c.extra, tags)
		if err != nil {
			bslog.Errorf("Unable to join tags: %v", err)
		} else {
			extra = &newExtra
		}
	}
	return extra
}

func pruneTags(input []string) []string {
	tags := []string{}
	for _, t := range input {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		tags = append(tags, t)
	}
	return tags
}

func newExtraWithTags(orig json.RawMessage, tags []string) (json.RawMessage, error) {
	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(orig), &decoded); err != nil {
		return nil, err
	}

	decoded["_tags"] = strings.Join(tags, ",")
	extra, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(extra), nil
}

func (c *InfoClient) configGelfExtraTags() {
	extra := config.StringEnvOrDefault("", "LOG_GELF_EXTRA_TAGS")
	if extra != "" {
		c.decodedExtra = map[string]string{}
		if err := json.Unmarshal([]byte(extra), &c.decodedExtra); err != nil {
			bslog.Warnf("unable to parse gelf extra tags: %s", err)
		} else {
			c.extra = json.RawMessage(extra)
		}
	}
}

func (c *Container) Stats() (*docker.Stats, error) {
	statsCh := make(chan *docker.Stats, 1)
	errCh := make(chan error, 1)
	opts := docker.StatsOptions{
		ID:      c.ID,
		Stream:  false,
		Stats:   statsCh,
		Timeout: 10 * time.Second,
	}
	go func() {
		defer close(errCh)
		err := c.client.client.Stats(opts)
		if err != nil {
			errCh <- err
		}
	}()
	err := <-errCh
	if err != nil {
		return nil, err
	}
	return <-statsCh, nil
}

// HasEnvs checks if the container has the requiredEnvs variables set
func (c *Container) HasEnvs(requiredEnvs []string) bool {
	for _, env := range requiredEnvs {
		hasEnv := false
		for _, val := range c.Config.Env {
			if strings.HasPrefix(val, env) {
				hasEnv = true
				break
			}
		}
		if !hasEnv {
			return false
		}
	}
	return true
}

func (c *Container) IsIsolated() bool {
	isIsolated, ok := c.GetLabelAny(labelIsIsolated...)
	return ok && isIsolated == "true"
}

// GetLabelAny returns the first label value that exists with given names
func (c *Container) GetLabelAny(names ...string) (string, bool) {
	for _, n := range names {
		if label, ok := c.Config.Labels[n]; ok {
			return label, true
		}
	}
	return "", false
}

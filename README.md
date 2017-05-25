# bs

[![Build Status](https://travis-ci.org/tsuru/bs.png?branch=master)](https://travis-ci.org/tsuru/bs)

bs (or big sibling) is a tsuru component, responsible for reporting
information on application containers, this information include application
logs, metrics and unit status.

bs runs inside a dedicated container on each Docker node and collects
information about all its sibling containers running on the same Docker node.

It also creates a syslog server, which is responsible for receiving all logs
from sibling containers. bs will then send these log entries to tsuru API and
is also capable of forwarding log entries to multiple remote syslog endpoints.

The sections below describe in details all the features of bs. The
[configuration](https://github.com/tsuru/tsuru/blob/master/docs/reference/config.rst#dockerbsimage)
reference contains more information on settings that control the way bs behaves.

## Status Reporting

bs communicates with the Docker API to collect information about containers,
and report them to the tsuru API. The bs "component" responsible for that is
the status reporter (or simply reporter).

The status reporter can connect to the Docker API through TCP or Unix Socket.
It's recommended to use Unix socket, so application containers can't talk to
the Docker API. In order to do that, the `docker:bs:socket` configuration
entry must be defined to the path of Docker socket in the Docker node. If this
setting is not defined, bs will use the TCP endpoint.

After collecting the data in the Docker API, the reporter will send it to the
tsuru API, and may take a last action before exiting: it can detect and kill
zombie containers, i.e. application containers that are running, but are not
known by tsuru. It doesn't mess with any container not managed by tsuru.

## Logging

bs can act as syslog server receiving logs from all containers and
distributing log messages to other syslog servers and the tsuru API.

When using tsuru default configuration every container started by tsuru will
be configured to send logs to the bs container on the same node using the
syslog protocol.

When receiving the logs, bs will forward them to the tsuru API, so users can
check their logs using the `tsuru app-log` command. It can also forward the
logs to other syslog servers, using the [configuration options described
below](#log_backends).

## Metrics

bs also collect metrics from containers and it's own host and send them to a
metric database backend. By default, bs will collect and report metrics from
all running containers, including its own container, this behavior can be
changed [by environment variable](#container_selection_env). Currently the
supported backend is `logstash`.

The following metrics are collected from containers:

* cpu_max
* mem_max
* mem_pct_max
* mem_limit
* swap
* swap_limit
* netrx
* nettx

The following metrics are collected from bs's own host:

* cpu (user, system, idle, stolen and wait percentages)
* mem (total, used and free)
* swap (total, used and free)
* disk (total, used and free)
* load (one, five and fifteen minutes)
* net (bytes received and sent)
* uptime (seconds)

To be able to collect host metrics, the proc filesystem (`/proc`) must be
mounted as a volume inside *bs* container and the `HOST_PROC` environment
variable must be set to it's path, tsuru currently [injects this
environ](https://github.com/tsuru/bs#injected-environment-variables). The
metric backend is configured by setting some enviroment variables in the *bs*
container. For more details check the [bs enviroment
variables](https://github.com/tsuru/bs#environment-variables).

## Environment Variables

It's possible to set environment variables in started bs containers. This can
be done using the `tsuru-admin node-container-update big-sibling --env
NAME=VALUE` command.

Some variables can be used to configure how the default bs application will
behave. A custom bs image can also make use of set variables to change their
behavior.

### LOG_BACKENDS

Comma separated list of which log backends are enabled. Currently possible
options are `tsuru`, `syslog` and `none`. Default value is `tsuru,syslog`
enabling both available backends.

Each backend has it's own possible config variables described in the next
sections.

### `tsuru` backend

Enabling `tsuru` log backend will send all received messages to tsuru api
server. The `tsuru app-log` command will only work if this backend is enabled.

#### LOG_TSURU_BUFFER_SIZE

`LOG_TSURU_BUFFER_SIZE` is the buffer size for log messages on this backend.
Default value is 1000000. Messages will be dropped if the buffer is full.

#### LOG_TSURU_PING_INTERVAL

`LOG_TSURU_PING_INTERVAL` is the interval in seconds between websocket Ping
frames sent to tsuru API server. This works as a heartbeat to check if the API
is still alive. Default value is 30 seconds.

#### LOG_TSURU_PONG_INTERVAL

`LOG_TSURU_PONG_INTERVAL` is the interval in seconds tsuru will wait for a
response Pong frame from a tsuru API server, after the Ping message is sent.
If a Pong message is not received in this interval the websocket connection
will be reopened. Default value is 4 times the value of
`LOG_TSURU_PING_INTERVAL`.

### `syslog` backend

Enabling `syslog` log backend will allow bs to forward all received logs to
other syslog servers. For this to work, at least one server must be set in
`LOG_SYSLOG_FORWARD_ADDRESSES`.

#### LOG_SYSLOG_BUFFER_SIZE

`LOG_SYSLOG_BUFFER_SIZE` is the buffer size for log messages on this backend.
Default value is 1000000. Messages will be dropped if the buffer is full.

#### LOG_SYSLOG_FORWARD_ADDRESSES (Previously SYSLOG_FORWARD_ADDRESSES)

`LOG_SYSLOG_FORWARD_ADDRESSES` is a comma separated list of SysLog endpoints
to which bs will forward the logs from Docker containers. Log entries will be
rewritten to properly identify the application and process responsible for the
entry. The default value is an empty string, which means that bs will not
forward logs to any syslog server, only to tsuru API.

#### LOG_SYSLOG_TIMEZONE (Previously SYSLOG_TIMEZONE)

`LOG_SYSLOG_TIMEZONE` which timezone to use when forwarding log to SysLog
servers. The timezone format must be a location existing in the [IANA Time
Zone database](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)


#### LOG_SYSLOG_MESSAGE_EXTRA_START and LOG_SYSLOG_MESSAGE_EXTRA_END
`LOG_SYSLOG_MESSAGE_EXTRA_{START,END}` are variables that can contain any text
to be added to the start or to the end of the forwarded syslog message. bs will
expand environment variables present in these messages during startup.

### STATUS_INTERVAL

`STATUS_INTERVAL` is the interval in seconds between status collecting and
reporting from bs to the tsuru API. The default value is 60 seconds.

### METRICS_INTERVAL

`METRICS_INTERVAL` is the interval in seconds between metrics collecting and
reporting from bs to the metric backend. The default value is 60 seconds.

### METRICS_BACKEND

`METRICS_BACKEND` is the metric backend. Currently the supported backend is
`logstash`.

### METRICS_LOGSTASH_CLIENT

`METRICS_LOGSTASH_CLIENT` is the client name used to identify who is sending
the metric. The default value is `tsuru`.

### METRICS_LOGSTASH_PORT

`METRICS_LOGSTASH_PORT` is the `Logstash` port. The default value is `1984`.

### METRICS_LOGSTASH_HOST

`METRICS_LOGSTASH_HOST` is the `Logstash` host. The default value is
`localhost`.

### METRICS_LOGSTASH_PROTOCOL

`METRICS_LOGSTASH_PROTOCOL` is the `Logstash` protocol. Supported protocols
are `udp` and `tcp`. The default value is `udp`.

### METRICS_NETWORK_INTERFACE

`METRICS_NETWORK_INTERFACE` is the `Network Interface` host. The default value is `eth0`.

### METRICS_ELASTICSEARCH_HOST

`METRICS_ELASTICSEARCH_HOST` is the `Elastisearch` host. This environ is used
by [tsuru-dashboard](https://github.com/tsuru/tsuru-dashboard) to show
graphics with the metrics data.

### CONTAINER_SELECTION_ENV

`CONTAINER_SELECTION_ENV` is the environment variable that needs to be set on
containers to have their metrics collected and reported by *bs*. *By default,
bs will report metrics from every container running on its host*, including
itself. For example, if `CONTAINER_SELECTION_ENV=TSURU_APPNAME` *bs* will only
report metrics from containers that have the `TSURU_APPNAME` environ (tsuru
application containers).

### BS_DEBUG

`BS_DEBUG` is a boolean value used to determine whether debug logs will be
printed. The default value is `false`.

### HOSTCHECK_BASE_CONTAINER_NAME

`HOSTCHECK_BASE_CONTAINER_NAME` is the container name from where bs will
extract the image name used to try creating a new container. The purpose of
creating this container is simply checking if docker is working correctly. If
this value is empty tsuru will try to use the same image used to start the bs
container.

### HOSTCHECK_EXTRA_PATHS

`HOSTCHECK_EXTRA_PATHS` is a comma separated list of paths where bs will try
to write a test file to check whether the filesystem is writable. If not set
tsuru will only try to write to `/`.

### HOSTCHECK_TIMEOUT

`HOSTCHECK_TIMEOUT` is the timeout, in seconds, for each check done on the
host. If the check takes more than the time specified in this environment
variable, the check will be considered a failure. The default value is `0`,
which means no timeout.

## Injected Environment Variables

Tsuru will inject some environment variables when starting the bs container.
Those *should not* be changed by the `tsuru-admin node-container-update`
command and are documented here for completeness and development purposes.

### TSURU_ENDPOINT

`TSURU_ENDPOINT` is the address to the tsuru api where logs will be forwarded
when using `tsuru` as a log backend and where container status are going to be
reported to.

### DOCKER_ENDPOINT

`DOCKER_ENDPOINT` is the docker endpoint from where the container metrics are
going to be collected from. The default value is "unix:///var/run/docker.sock".

### SYSLOG_LISTEN_ADDRESS

`SYSLOG_LISTEN_ADDRESS` is the local syslog server address that other
container logs are being sent to be forwarded by bs.

### HOST_PROC

`HOST_PROC` is the path to the volume where *bs* host `/proc` was mounted in
the *bs* container.

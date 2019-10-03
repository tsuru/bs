# Copyright 2016 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.13-alpine3.10 as builder

COPY . /go/src/github.com/tsuru/bs
WORKDIR /go/src/github.com/tsuru/bs
RUN go build

FROM alpine:3.10
RUN  apk update && apk add conntrack-tools ca-certificates tzdata && rm -rf /var/cache/apk/*
COPY --from=builder /go/src/github.com/tsuru/bs/bs /bin/bs
ENTRYPOINT ["/bin/bs"]

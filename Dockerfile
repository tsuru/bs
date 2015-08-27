# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM golang:1.4
RUN apt-get update && apt-get install -y conntrack
RUN go get github.com/tools/godep
ADD . /go/src/github.com/tsuru/bs
RUN cd /go/src/github.com/tsuru/bs && godep restore ./... && go install
ENTRYPOINT ["/go/bin/bs"]

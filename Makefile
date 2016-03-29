# Copyright 2016 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

test:
	go clean ./...
	go test  ./... -check.vv

run:
	go run main.go

_build:
	go build -ldflags "-linkmode external -extldflags -static"

publish-local: _build
	docker build -t 127.0.0.1:5000/tsuru/bs .
	docker push 127.0.0.1:5000/tsuru/bs


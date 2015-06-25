# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

TAG ?= tsuru/bs

docker_image:
	go build
	docker build -t $(TAG) .
	rm bs

# Copyright 2016 tsuru authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

test:
	go clean ./...
	go test  ./... -check.vv

dirs = `go list -f '{{.Dir}}/*.go' ./... | grep -v vendor`
format:
	gofmt -s -w $(dirs)
	goimports -srcdir . -w $(dirs)

check-format:
	bash -c 'result=$$(gofmt -s -l $(dirs)); test -z $$result || (echo $$result && exit 1)'
	go get golang.org/x/tools/cmd/goimports
	bash -c 'result=$$(goimports -srcdir . -l $(dirs)); test -z $$result || (echo $$result && exit 1)'

run:
	go run main.go

lint:
	golangci-lint run

publish-local:
	docker build -t 127.0.0.1:5000/tsuru/bs .
	docker push 127.0.0.1:5000/tsuru/bs

viewparser:
	@ragel -pV log/parser.rl > parser.dot
	@dot -Tpng parser.dot > parser.png
	@rm parser.dot
	@open -W parser.png
	@rm parser.png

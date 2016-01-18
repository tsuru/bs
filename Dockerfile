# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM alpine:3.2
RUN  apk update && apk add conntrack-tools ca-certificates tzdata && rm -rf /var/cache/apk/*
ADD  bs /bin/bs
ENTRYPOINT ["/bin/bs"]

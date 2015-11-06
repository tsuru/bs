# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM debian
RUN  apt-get update && \
     apt-get install -y conntrack wget && \
     wget -O /bin/bs https://s3.amazonaws.com/tsuru/bs/bs && \
     apt-get remove -y --purge wget && \
     apt-get autoremove -y --purge && \
     rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/* /var/lib/dpkg/info/*
RUN  chmod +x /bin/bs
ENTRYPOINT ["/bin/bs"]

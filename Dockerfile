# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM cirros
RUN  curl -Lo /bin/bs https://s3.amazonaws.com/tsuru/bs/bs
ENTRYPOINT ["/bin/bs"]

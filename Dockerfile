# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM ubuntu:14.04
ADD bs /usr/bin/bs
ENTRYPOINT ["/usr/bin/bs"]

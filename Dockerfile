# Copyright 2015 bs authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

FROM tsuru/bs-base
ADD  bs /bin/bs
ENTRYPOINT ["/bin/bs"]

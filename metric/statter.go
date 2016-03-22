// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

type statter interface {
	Send(app, hostname, process, key string, value interface{}) error
	SendConn(app, hostname, process, host string) error
    SendSys(hostname, key string, value interface{}) error
}

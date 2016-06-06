// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package node

import (
	"net"
	"strings"
)

func GetNodeAddrs() ([]string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	addrsRet := make([]string, 0, len(addrs))
	for _, a := range addrs {
		var addrStr string
		switch v := a.(type) {
		case *net.IPNet:
			addrStr = v.IP.String()
		case *net.IPAddr:
			addrStr = v.IP.String()
		default:
			ip := net.ParseIP(strings.SplitN(a.String(), "/", 2)[0])
			if ip != nil {
				addrStr = ip.String()
			}
		}
		if addrStr != "" {
			addrsRet = append(addrsRet, addrStr)
		}
	}
	return addrsRet, nil
}

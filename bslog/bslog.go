// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bslog

import (
	"fmt"
	"log"
)

var Debug bool

func Debugf(msg string, params ...interface{}) {
	if Debug {
		printf("DEBUG", msg, params...)
	}
}

func Warnf(msg string, params ...interface{}) {
	printf("WARNING", msg, params...)
}

func Errorf(msg string, params ...interface{}) {
	printf("ERROR", msg, params...)
}

func Fatalf(msg string, params ...interface{}) {
	log.Fatalf(msg, params...)
}

func printf(level string, msg string, params ...interface{}) {
	msg = fmt.Sprintf("[%s] %s", level, msg)
	log.Printf(msg, params...)
}

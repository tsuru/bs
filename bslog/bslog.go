// Copyright 2015 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bslog

import (
	"fmt"
	"log"
	"os"
)

var Debug bool

var Logger = log.New(os.Stderr, "", log.LstdFlags)

func Debugf(msg string, params ...interface{}) {
	if Debug {
		logPrintf("DEBUG", msg, params...)
	}
}

func Warnf(msg string, params ...interface{}) {
	logPrintf("WARNING", msg, params...)
}

func Errorf(msg string, params ...interface{}) {
	logPrintf("ERROR", msg, params...)
}

func Fatalf(msg string, params ...interface{}) {
	Logger.Fatalf(msg, params...)
}

func logPrintf(level string, msg string, params ...interface{}) {
	msg = fmt.Sprintf("[%s] %s", level, msg)
	Logger.Printf(msg, params...)
}

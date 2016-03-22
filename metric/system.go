// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
    "github.com/elastic/gosigar"
    "os"      
)

func getSystemLoad() (map[string]float, error){
    concreteSigar := gosigar.ConcreteSigar{}
    
    load, err := concreteSigar.GetLoadAverage()
    if err != nil {
        return nil, err
    }
    
    stats := map[string]float{
        "load1": float(load.One),
        "load5": float(load.Five),
        "load15": float(load.Fifteen),
    }
    
    return stats, nil
    
}

func getHostname() (string, error) {
    hostname, err := os.Hostname()
    if err != nil {
        return "", err
    }
    return hostname, nil
}
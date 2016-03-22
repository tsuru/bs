// Copyright 2016 bs authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metric

import (
    "github.com/elastic/gosigar"
    "os"      
)

func getSystemMetrics() ([]map[string]float, error) {
    collectors := []func()(map[string]float,error){
        getSystemLoad, 
        getSystemMem,
        getSystemSwap,
        getFileSystemUsage,
    }
    var metrics []map[string]float
    for _, collector := range collectors {
        metric, err := collector()
        if err != nil {
            return nil, err
        }
        metrics = append(metrics, metric)
    }
    return metrics, nil
}

func getSystemLoad() (map[string]float, error) {
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

func getSystemMem() (map[string]float, error) {
    concreteSigar := gosigar.ConcreteSigar{}
    mem, err := concreteSigar.GetMem()
    if err != nil {
        return nil, err
    }
    stats := map[string]float{
        "mem_total": float(mem.Total),
        "mem_used": float(mem.Used),
        "mem_free": float(mem.Free),
    }
    return stats, nil
}

func getSystemSwap() (map[string]float, error) {
    concreteSigar := gosigar.ConcreteSigar{}
    swap, err := concreteSigar.GetSwap()
    if err != nil {
        return nil, err
    }
    stats := map[string]float{
        "swap_total": float(swap.Total),
        "swap_used": float(swap.Used),
        "swap_free": float(swap.Free),
    }
    return stats, nil
}

func getFileSystemUsage() (map[string]float, error) {
    concreteSigar := gosigar.ConcreteSigar{}
    disk, err := concreteSigar.GetFileSystemUsage("/")
    if err != nil {
        return nil, err
    }
    stats := map[string]float{
        "disk_total": float(disk.Total),
        "disk_used": float(disk.Used),
        "disk_free": float(disk.Free),
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
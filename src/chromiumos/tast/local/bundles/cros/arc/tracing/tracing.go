// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tracing

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

type tracingEntry struct {
	hits      int64
	mcs       int64
	kbytes    int64
	pid       int64
	pname     string
	fs        string
	operation string
	inode     int64
	opPath    string
}

type aggregatedStat struct {
	name   string
	hits   int64
	mcs    int64
	kbytes int64
}

type aggregatorFunc func(entry tracingEntry) string

func totalAggregator(entry tracingEntry) string {
	return "Total"
}

func fsAggregator(entry tracingEntry) string {
	return entry.fs
}

func fsOpAggregator(entry tracingEntry) string {
	return entry.fs + "," + entry.operation
}

func fileAggregator(entry tracingEntry) string {
	return entry.fs + "," + entry.opPath + "," + strconv.FormatInt(entry.inode, 10)
}

func mcsToString(mcs int64) string {
	return fmt.Sprintf("%.3fs", float64(mcs)/(1000000.0))
}

func kbytesToString(kbytes int64) string {
	if kbytes >= 100*1024 {
		return fmt.Sprintf("%.3fG", float64(kbytes)/(1024.0*1024.0))
	}
	if kbytes >= 100 {
		return fmt.Sprintf("%.3fM", float64(kbytes)/(1024.0))
	}
	return fmt.Sprintf("%.3fK", float64(kbytes)*0.1)
}

// AnylyzeTracing takes output of custom kernel profiling, processes it and report the result
// as CVS file. |aggregatorType| defines the aggregation type and should be one of
//
//	total - total aggregation, expected one output line
//	fs - aggregation per file system, for example vda. It includes all operation on this fs
//	fsop - similar to above but provide extra separation per operation like open, read, page_fault, write
//	file - aggregation per particular file
//
// |threshold| specifies minimum time to report in mcs.
func AnylyzeTracing(content, aggregatorType string, threshold int64) (string, error) {
	var aggregator aggregatorFunc
	if aggregatorType == "total" {
		aggregator = totalAggregator
	} else if aggregatorType == "fs" {
		aggregator = fsAggregator
	} else if aggregatorType == "fsop" {
		aggregator = fsOpAggregator
	} else if aggregatorType == "file" {
		aggregator = fileAggregator
	} else {
		return "", errors.Errorf("unknown aggregator %s, expected total|fs|fsop|file", aggregatorType)
	}

	dict := map[string]aggregatedStat{}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		// fmt.Println(line)
		fields := strings.Split(line, ",")
		if len(fields) != 9 {
			return "", errors.Errorf("expected 9 but got %d fields in line %s", len(fields), line)
		}

		hits, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
		if err != nil || hits < 0 {
			return "", errors.Errorf("failed to parse hits %s", fields[0])
		}
		mcs, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64)
		if err != nil || mcs < 0 {
			return "", errors.Errorf("failed to parse mcs %s", fields[1])
		}
		kbytes, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil || kbytes < 0 {
			return "", errors.Errorf("failed to parse kbytes %s", fields[2])
		}
		pid, err := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64)
		if err != nil || pid < 0 {
			return "", errors.Errorf("failed to parse pid %s", fields[3])
		}
		pname := strings.TrimSpace(fields[4])
		fs := strings.TrimSpace(fields[5])
		operation := strings.TrimSpace(fields[6])
		inode, err := strconv.ParseInt(strings.TrimSpace(fields[7]), 10, 64)
		if err != nil || hits < 0 {
			return "", errors.Errorf("failed to parse inode %s", fields[7])
		}
		opPath := strings.TrimSpace(fields[8])

		entry := tracingEntry{}
		entry.hits = hits
		entry.mcs = mcs
		entry.kbytes = kbytes
		entry.pid = pid
		entry.pname = pname
		entry.fs = fs
		entry.operation = operation
		entry.inode = inode
		entry.opPath = opPath

		key := aggregator(entry)
		if stat, ok := dict[key]; ok {
			stat.hits += entry.hits
			stat.mcs += entry.mcs
			stat.kbytes += entry.kbytes
			dict[key] = stat
		} else {
			stat := aggregatedStat{}
			stat.name = key
			stat.hits = entry.hits
			stat.mcs = entry.mcs
			stat.kbytes = entry.kbytes
			dict[key] = stat
		}
	}

	result := ""
	var stats []aggregatedStat
	for _, stat := range dict {
		stats = append(stats, stat)
	}

	sort.SliceStable(stats, func(i, j int) bool {
		return stats[i].mcs > stats[j].mcs
	})

	fmt.Printf("key, hits, time, volume\n")
	for _, stat := range stats {
		if stat.mcs < threshold {
			break
		}
		result += fmt.Sprintf("%s, %d, %s, %s\n", stat.name, stat.hits, mcsToString(stat.mcs), kbytesToString(stat.kbytes))
	}

	return result, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage %s tracing_file aggregator total|fs|fsop|file\n", os.Args[0])
		return
	}

	content, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	result, err := AnylyzeTracing(string(content), os.Args[2], 1000)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result)
}

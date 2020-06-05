// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package memory provides a mechanism for reading ChromeOS's memory pressure
// level.
package memory

import (
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

// readFirstUint reads the first unsigned integer from a file.
func readFirstUint(f string) (uint, error) {
	// Files will always just be a single line, so it's OK to read everything.
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return 0, err
	}
	firstString := strings.Split(strings.TrimSpace(string(data)), " ")[0]
	firstUint, err := strconv.ParseUint(firstString, 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return uint(firstUint), nil
}

// Available returns the amount of currently available memory in MB.
func Available() (uint, error) {
	const availableMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	return readFirstUint(availableMemorySysFile)
}

// CriticalMargin returns the available memory threshold below which the system
// is under critical memory pressure.
func CriticalMargin() (uint, error) {
	const marginMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/margin"
	return readFirstUint(marginMemorySysFile)
}

const zoneInfoFile = "/proc/zoneinfo"

var zoneinfoRE = regexp.MustCompile(`(?m)^Node +\d+, +zone +([^ ])+
(?:(?: +pages free +(\d+)
 +min +(\d+)
 +low +(\d+)
)|(?: +.*
))*`)

// min returns the smaller of two uint64.
func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

type zoneInfo struct {
	name string
	free uint64
	low  uint64
	min  uint64
}

func readZoneInfo() ([]zoneInfo, error) {
	data, err := ioutil.ReadFile(zoneInfoFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open zoneinfo")
	}

	matches := zoneinfoRE.FindAllStringSubmatch(string(data), -1)
	if matches == nil {
		return nil, errors.Wrap(err, "failed to parse zoneinfo")
	}

	var infos []zoneInfo
	for _, match := range matches {
		free, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zone free")
		}
		minWatermark, err := strconv.ParseUint(match[3], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zone min")
		}
		lowWatermark, err := strconv.ParseUint(match[4], 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse zone low")
		}
		const bytesPerPage = 4096
		infos = append(infos, zoneInfo{
			name: match[1],
			free: free * bytesPerPage,
			low:  lowWatermark * bytesPerPage,
			min:  minWatermark * bytesPerPage,
		})
	}
	return infos, nil
}

// ReadMinDistanceToZoneMin reads the smallest distance between a zone's free
// count and its min watermark. Small or empty zones are ignored.
// returns the distance in bytes.
func ReadMinDistanceToZoneMin() (int64, error) {
	infos, err := readZoneInfo()
	if err != nil {
		return 0, err
	}

	var distance int64 = math.MaxInt64
	for _, info := range infos {
		// Ignore small or empty zones, we don't want to throttle allocations
		// based on a small distance from a small or empty zone.
		const smallZoneLimit = 4096000
		if info.low > smallZoneLimit {
			distance = min(distance, int64(info.free)-int64(info.min))
		}
	}
	if distance == math.MaxInt64 {
		return 0, errors.Wrap(err, "no non-empty zones found")
	}
	return distance, nil
}

// AnyZoneLow returns true if any kernel memory zone is below the low watermark.
func AnyZoneLow() (bool, error) {
	infos, err := readZoneInfo()
	if err != nil {
		return false, err
	}

	for _, info := range infos {
		if info.free < info.low {
			return true, nil
		}
	}
	return false, nil
}

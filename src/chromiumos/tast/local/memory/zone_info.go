// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"io/ioutil"
	"regexp"
	"strconv"

	"chromiumos/tast/errors"
)

const zoneInfoFile = "/proc/zoneinfo"

var zoneInfoRE = regexp.MustCompile(`(?m)^Node +\d+, +zone +([^ ])+
(?:(?: +pages free +(\d+)
 +min +(\d+)
 +low +(\d+)
)|(?: +.*
))*`)

// ZoneInfo contains the values of counters from one zone in /proc/zoneinfo.
// All sizes are in bytes.
type ZoneInfo struct {
	Name string
	Free uint64
	Low  uint64
	Min  uint64
}

// ReadZoneInfo parses /proc/zoneinfo into a slice of ZoneInfo structures.
func ReadZoneInfo() ([]ZoneInfo, error) {
	data, err := ioutil.ReadFile(zoneInfoFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open zoneinfo")
	}

	matches := zoneInfoRE.FindAllStringSubmatch(string(data), -1)
	if matches == nil {
		return nil, errors.Wrap(err, "failed to parse zoneinfo")
	}

	var infos []ZoneInfo
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
		infos = append(infos, ZoneInfo{
			Name: match[1],
			Free: free * PageBytes,
			Low:  lowWatermark * PageBytes,
			Min:  minWatermark * PageBytes,
		})
	}
	return infos, nil
}

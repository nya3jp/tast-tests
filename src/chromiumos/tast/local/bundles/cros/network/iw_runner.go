// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network contains local Tast tests that exercise the Chrome OS network stack.
package network

import (
	"regexp"
	"strings"
)

/*
This file serves as a wrapper to allow tast tests to query 'iw' for network device characteristics.
iw_runner.go is the analog of {@link iw_runner.py} in the autotest suite.
*/

func _get_all_link_keys(link_information string) map[string]string {
	/*
			Parses link or station dump output for link key value pairs.

		    Link or station dump information is in the format below:

		    Connected to 74:e5:43:10:4f:c0 (on wlan0)
		          SSID: PMKSACaching_4m9p5_ch1
		          freq: 5220
		          RX: 5370 bytes (37 packets)
		          TX: 3604 bytes (15 packets)
		          signal: -59 dBm
		          tx bitrate: 13.0 MBit/s MCS 1

		          bss flags:      short-slot-time
		          dtim period:    5
		          beacon int:     100

		    @param link_information: string containing the raw link or station dump
		        information as reported by iw. Note that this parsing assumes a single
		        entry, in the case of multiple entries (e.g. listing stations from an
		        AP, or listing mesh peers), the entries must be split on a per
		        peer/client basis before this parsing operation.
		    @return a map containing all the link key/value pairs.

	*/
	link_key_value_pairs := make(map[string]string)
	r := regexp.MustCompile("^[[:space:]]+(.*):[[:space:]]+(.*)$")
	for _, link_key := range strings.Split(link_information, "\n") {
		if r.MatchString(link_key) {
			match_group := r.FindStringSubmatch(link_key)
			link_key_value_pairs[match_group[1]] = match_group[2]
		}
	}
	return link_key_value_pairs
}

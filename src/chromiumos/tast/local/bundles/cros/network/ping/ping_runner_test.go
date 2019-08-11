// Copyright 2019The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ping contains utility functions to wrap around the ping program.
package ping

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"strings"
	"testing"
)

func TestCfgToArgs(t *testing.T) {
	// The interval float parameter will display up to 6 decimal places
	strCmp := "-c 7 -s 3 -i 0.123456 -I wlan0 -Q 0x10 1.2.3.4"
	cfg := Config{TargetIP: "1.2.3.4", Count: 7, Size: 3, Interval: 0.123456, SourceIface: "wlan0", QOS: "vo"}
	out, err := cfgToArgs(cfg)
	if err != nil {
		t.Error(err.Error())
	}
	res := strings.Join(out, " ")
	if res != strCmp {
		t.Error(fmt.Sprintf("cfgToArgs: outputs differ. Got %s, expected %s", res, strCmp))
	}
}

func TestParseOutput(t *testing.T) {
	strCmp := `PING 8.8.8.8 (8.8.8.8): 56 data bytes
        64 bytes from 8.8.8.8: icmp_seq=0 ttl=57 time=3.770 ms
        64 bytes from 8.8.8.8: icmp_seq=1 ttl=57 time=4.165 ms

        --- 8.8.8.8 ping statistics ---
        3 packets transmitted, 2 packets received, 33.33% packet loss
        round-trip min/avg/max/stddev = 3.770/4.279/4.901/0.469 ms`
	prCmp := &Result{Sent: 3, Received: 2, Loss: 33.33, MinLatency: 3.770, AvgLatency: 4.279, MaxLatency: 4.901, DevLatency: .469}
	out, err := parseOutput(strCmp)
	if err != nil {
		t.Error(err.Error())
	}
	if diff := cmp.Diff(prCmp, out); diff != "" {
		t.Error("parseOutput returned unexpected result; diff\n ", diff)
	}
}

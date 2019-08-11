// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ping

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCfgToArgs(t *testing.T) {
	// The interval float parameter will display up to 6 decimal places
	expected := []string{"-c", "7", "-s", "3", "-i", "0.123456", "-I", "wlan0", "-Q", "0x10", "1.2.3.4"}
	cfg := Config{TargetIP: "1.2.3.4", Count: 7, Size: 3, Interval: 0.123456, SourceIface: "wlan0", QOS: "vo"}
	args, err := cfgToArgs(cfg)
	if err != nil {
		t.Error(err.Error())
	}
	if !reflect.DeepEqual(expected, args) {
		t.Error(fmt.Sprintf("cfgToArgs: outputs differ. Got %v, expected %v", args, expected))
	}
}

func TestParseOutput(t *testing.T) {
	testInputs := []string{
		`PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=58 time=2.81 ms
ping: sendmsg: Network is unreachable
ping: sendmsg: Network is unreachable
64 bytes from 8.8.8.8: icmp_seq=4 ttl=58 time=2.82 ms
64 bytes from 8.8.8.8: icmp_seq=5 ttl=58 time=1.71 ms

--- 8.8.8.8 ping statistics ---
5 packets transmitted, 3 received, 40% packet loss, time 12004ms
rtt min/avg/max/mdev = 1.717/2.451/2.826/0.520 ms`, // Output collected from rammus DUT.
		`PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=59 time=0.638 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=59 time=0.653 ms

--- 8.8.8.8 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1028ms
rtt min/avg/max/mdev = 0.638/0.645/0.653/0.026 ms`, // Output collected on host.
	}
	expected := []*Result{
		&Result{Sent: 5, Received: 3, Loss: 40, MinLatency: 1.717, AvgLatency: 2.451, MaxLatency: 2.826, DevLatency: .520},
		&Result{Sent: 2, Received: 2, Loss: 0, MinLatency: 0.638, AvgLatency: 0.645, MaxLatency: 0.653, DevLatency: .026},
	}
	for i := range testInputs {
		output, err := parseOutput(testInputs[i])
		if err != nil {
			t.Error(err.Error())
		}
		if !reflect.DeepEqual(expected[i], output) {
			t.Error("parseOutput returned unexpected result; diff=\n", cmp.Diff(expected[i], output))
		}
	}
}

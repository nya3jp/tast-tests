// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ping

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestOptions(t *testing.T) {
	cfg := &config{}
	opts := []Option{
		BindAddress(true),
		Count(3),
		Interval(0.5),
		Size(100),
		SourceIface("eth0"),
		QOS(QOSVI),
	}
	expected := &config{
		BindAddress: true,
		Count:       3,
		Interval:    0.5,
		Size:        100,
		SourceIface: "eth0",
		QOS:         QOSVI,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("options apply failed. Got %v, expected %v", cfg, expected)
	}
}

func TestCfgToArgs(t *testing.T) {
	// The interval float parameter will display up to 6 decimal places
	expected := []string{"-B", "-c", "7", "-s", "3", "-i", "0.123456", "-I", "wlan0", "-Q", "0x10", "1.2.3.4"}
	cfg := &config{BindAddress: true, Count: 7, Size: 3, Interval: 0.123456, SourceIface: "wlan0", QOS: QOSVO}
	args, err := cfgToArgs("1.2.3.4", cfg)
	if err != nil {
		t.Error(err.Error())
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("cfgToArgs: outputs differ. Got %v, expected %v", args, expected)
	}
}

func TestParseOutput(t *testing.T) {
	testcases := []struct {
		input  string
		expect *Result
	}{
		// Output collected from rammus DUT.
		{
			input: `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=58 time=2.81 ms
ping: sendmsg: Network is unreachable
ping: sendmsg: Network is unreachable
64 bytes from 8.8.8.8: icmp_seq=4 ttl=58 time=2.82 ms
64 bytes from 8.8.8.8: icmp_seq=5 ttl=58 time=1.71 ms

--- 8.8.8.8 ping statistics ---
5 packets transmitted, 3 received, 40% packet loss, time 12004ms
rtt min/avg/max/mdev = 1.717/2.451/2.826/0.520 ms`,
			expect: &Result{Sent: 5, Received: 3, Loss: 40, MinLatency: 1.717, AvgLatency: 2.451, MaxLatency: 2.826, DevLatency: .520},
		},
		// Output collected in chroot.
		{
			input: `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=59 time=0.638 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=59 time=0.653 ms

--- 8.8.8.8 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1028ms
rtt min/avg/max/mdev = 0.638/0.645/0.653/0.026 ms`,
			expect: &Result{Sent: 2, Received: 2, Loss: 0, MinLatency: 0.638, AvgLatency: 0.645, MaxLatency: 0.653, DevLatency: .026},
		},
	}
	for i := range testcases {
		output, err := parseOutput(testcases[i].input)
		if err != nil {
			t.Errorf("testcase %d failed with err=%s", i, err.Error())
		}
		if diff := cmp.Diff(output, testcases[i].expect); diff != "" {
			t.Errorf("testcase %d returned unexpected result; diff=\n%s", i, diff)
		}
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ip

import (
	"context"
	"net"
	"reflect"
	"testing"
)

// stubCmdRunner is a simple stub of CmdRunner which always returns the given content
// as command output. This is useful for testing some simple parsing that is not
// extracted as an independent function.
type stubCmdRunner struct {
	out []byte
}

// Run is a noop stub which always returns nil.
func (r *stubCmdRunner) Run(ctx context.Context, cmd string, args ...string) error {
	return nil
}

// Output is a stub which pretends the command is executed successfully and prints
// the pre-assigned output.
func (r *stubCmdRunner) Output(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	return r.out, nil
}

func TestGetMAC(t *testing.T) {
	testcases := []struct {
		out        string
		expect     net.HardwareAddr
		shouldFail bool
	}{
		// Some invalid output.
		{
			out:        "",
			shouldFail: true, // Empty.
		},
		{
			out: `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			shouldFail: true, // Unmatched name.
		},
		// Valid case.
		{
			out: `2: wlan0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 1a:2b:3c:4d:5e:6f ff:ff:ff:ff:ff:ff
`,
			expect:     net.HardwareAddr{0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f},
			shouldFail: false,
		},
	}
	stub := &stubCmdRunner{}
	r := NewRunner(stub)
	for i, tc := range testcases {
		stub.out = []byte(tc.out)
		// Test MAC function.
		got, err := r.MAC(context.Background(), "wlan0")
		if tc.shouldFail {
			if err == nil {
				t.Errorf("case#%d should have error", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("case#%d failed with err=%v", i, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.expect) {
			t.Errorf("case#%d got MAC: %v, want: %v", i, got, tc.expect)
		}
	}
}

func TestShowLink(t *testing.T) {
	testcases := []struct {
		shouldFail bool
		out        string
		expect     []string
	}{
		// Invalid cases.
		{
			shouldFail: true, // Empty.
			out:        "",
		},
		{
			shouldFail: true, // Incomplete results.
			out:        `1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000`,
		},
		{
			shouldFail: true, // Unmatched name.
			out: `2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
		},
		{
			shouldFail: true, // Brief format not supported.
			out:        "wlan0             UP             01:02:03:04:05:06 <BROADCAST,MULTICAST,UP,LOWER_UP> \n",
		},

		// Valid cases.
		{
			shouldFail: false,
			out: `2: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{"wlan0", "UP", "01:02:03:04:05:06", "<BROADCAST,MULTICAST,UP,LOWER_UP>", ""},
		},
		{
			shouldFail: false,
			out: `2: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
`,
			expect: []string{"wlan0", "UP", "01:02:03:04:05:06", "<BROADCAST,MULTICAST,UP,LOWER_UP>", ""},
		},
		{
			shouldFail: false,
			out: `2: wlan0@: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
`,
			expect: []string{"wlan0", "UP", "01:02:03:04:05:06", "<BROADCAST,MULTICAST,UP,LOWER_UP>", ""},
		},
		{
			shouldFail: false,
			out: `2: wlan0@someAlias: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
`,
			expect: []string{"wlan0", "UP", "01:02:03:04:05:06", "<BROADCAST,MULTICAST,UP,LOWER_UP>", "someAlias"},
		},
	}
	stub := &stubCmdRunner{}
	r := NewRunner(stub)
	for i, tc := range testcases {
		stub.out = []byte(tc.out)
		// Test showLink function.
		got, err := r.showLink(context.Background(), "wlan0")
		if tc.shouldFail {
			if err == nil {
				t.Errorf("case#%d should have error", i)
			}
			continue
		}
		if err != nil {
			t.Errorf("case#%d failed with err=%v", i, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.expect) {
			t.Errorf("case#%d got MAC: %v, want: %v", i, got, tc.expect)
		}
	}
}

func TestLinkWithPrefix(t *testing.T) {
	testcases := []struct {
		prefix string
		out    string
		expect []string
	}{
		{
			prefix: "somePrefix",
			out:    "",
			expect: []string{},
		},
		{
			prefix: "somePrefix",
			out:    "wlan0             UP             01:02:03:04:05:06 <BROADCAST,MULTICAST,UP,LOWER_UP> \n",
			expect: []string{}, // Brief format not expected.
		},
		{
			prefix: "somePrefix",
			out: `2: wlan0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{}, // No matches.
		},
		{
			prefix: "somePrefix",
			out: `2: somePrefix: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{"somePrefix"}, // Name matches exactly.
		},
		{
			prefix: "somePrefix",
			out: `2: somePrefix123: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{"somePrefix123"}, // Name matches with prefix.
		},
		{
			prefix: "somePrefix",
			out: `2: somePrefix123@: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{"somePrefix123"}, // Can match even with empty alias.
		},
		{
			prefix: "somePrefix",
			out: `2: somePrefix123@someAlias: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 ff:ff:ff:ff:ff:ff
    altname enp2s0`,
			expect: []string{"somePrefix123"}, // Can match even with an alias.
		},
		{
			prefix: "veth",
			out: `
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: veth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 brd ff:ff:ff:ff:ff:ff
    altname enp2s0
3: eno2: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc fq_codel state DOWN mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 brd ff:ff:ff:ff:ff:ff
    altname enp0s31f6
`,
			expect: []string{"veth1"}, // Can match with multiple interfaces.
		},
		{
			prefix: "veth",
			out: `
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: veth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 brd ff:ff:ff:ff:ff:ff
    altname enp2s0
3: veth2: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc fq_codel state DOWN mode DEFAULT group default qlen 1000
    link/ether 01:02:03:04:05:06 brd ff:ff:ff:ff:ff:ff
    altname enp0s31f6
`,
			expect: []string{"veth1", "veth2"}, // Can match multiple ifaces.
		},
	}
	stub := &stubCmdRunner{}
	r := NewRunner(stub)
	for i, tc := range testcases {
		stub.out = []byte(tc.out)
		// Test showLink function.
		got, err := r.LinkWithPrefix(context.Background(), tc.prefix)
		if err != nil {
			t.Errorf("case#%d failed with err=%v", i, err)
			continue
		}
		if len(tc.expect) == 0 {
			if len(got) != 0 {
				t.Errorf("case#%d got: %v, want: %v", i, got, tc.expect)
			}
		} else if !reflect.DeepEqual(got, tc.expect) {
			t.Errorf("case#%d got: %v, want: %v", i, got, tc.expect)
		}
	}
}

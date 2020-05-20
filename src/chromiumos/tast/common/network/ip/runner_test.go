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
		// Some invalid jsons.
		{
			out:        "",
			shouldFail: true,
		},
		{
			out:        "[]",
			shouldFail: true, // No result.
		},
		{
			out:        "[{}]",
			shouldFail: true, // No "address" field.
		},
		{
			out:        `[{"ifindex":1,"ifname":"lo","flags":["LOOPBACK","UP","LOWER_UP"],"mtu":65536,"qdisc":"noqueue","operstate":"UNKNOWN","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"loopback","address":"00:00:00:00:00:00","broadcast":"00:00:00:00:00:00"},{"ifindex":3,"ifname":"wlan0","flags":["NO-CARRIER","BROADCAST","MULTICAST","UP"],"mtu":1500,"qdisc":"noqueue","operstate":"DOWN","linkmode":"DORMANT","group":"default","txqlen":1000,"link_type":"ether","address":"18:5e:0f:4e:23:43","broadcast":"ff:ff:ff:ff:ff:ff"},{"ifindex":4,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"pfifo_fast","operstate":"UP","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"ether","address":"58:ef:68:b4:08:b9","broadcast":"ff:ff:ff:ff:ff:ff"}]`,
			shouldFail: true, // Multiple results.
		},
		// Valid case.
		{
			out:        `[{"ifindex":4,"ifname":"eth0","flags":["BROADCAST","MULTICAST","UP","LOWER_UP"],"mtu":1500,"qdisc":"pfifo_fast","operstate":"UP","linkmode":"DEFAULT","group":"default","txqlen":1000,"link_type":"ether","address":"58:ef:68:b4:08:b9","broadcast":"ff:ff:ff:ff:ff:ff"}]`,
			expect:     net.HardwareAddr{0x58, 0xef, 0x68, 0xb4, 0x08, 0xb9},
			shouldFail: false,
		},
	}
	stub := &stubCmdRunner{}
	r := &Runner{cmd: stub}
	for i, tc := range testcases {
		stub.out = []byte(tc.out)
		// Test MAC function.
		got, err := r.MAC(context.Background(), "eth0")
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

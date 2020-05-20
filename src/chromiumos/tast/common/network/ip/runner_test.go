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
			out: `lo               UNKNOWN        00:00:00:00:00:00 <LOOPBACK,UP,LOWER_UP> 
eth0             UP             01:02:03:04:05:06 <BROADCAST,MULTICAST,UP,LOWER_UP> 
wlan0            DOWN           1a:2b:3c:4d:5e:6f <NO-CARRIER,BROADCAST,MULTICAST,UP> 
`,
			shouldFail: true, // Multiple results.
		},
		{
			out:        "eth0             UP             01:02:03:04:05:06 <BROADCAST,MULTICAST,UP,LOWER_UP> \n",
			shouldFail: true, // Unmatched name.
		},
		// Valid case.
		{
			out:        "wlan0            DOWN           1a:2b:3c:4d:5e:6f <NO-CARRIER,BROADCAST,MULTICAST,UP> \n",
			expect:     net.HardwareAddr{0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f},
			shouldFail: false,
		},
	}
	stub := &stubCmdRunner{}
	r := &Runner{cmd: stub}
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

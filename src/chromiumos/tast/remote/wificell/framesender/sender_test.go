// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package framesender

import (
	"context"
	"reflect"
	"testing"
)

func TestConfigToArgs(t *testing.T) {
	const iface = "iface"

	testcases := []struct {
		name   string
		c      *config
		expect []string
	}{
		{
			name: "basic",
			c: &config{
				t:     TypeBeacon,
				ch:    36,
				count: 1,
			},
			expect: []string{"-i", iface, "-t", "beacon", "-c", "36", "-n", "1"},
		},
		{
			name: "options",
			c: &config{
				t:          TypeBeacon,
				ch:         36,
				count:      3,
				ssidPrefix: "prefix",
				numBSS:     2,
				delay:      10,
				destMAC:    "01:02:03:04:05:06",
			},
			expect: []string{
				"-i", iface,
				"-t", "beacon",
				"-c", "36",
				"-n", "3",
				"-s", "prefix",
				"-b", "2",
				"-d", "10",
				"-a", "01:02:03:04:05:06",
			},
		},
	}

	// A dummy sender without a valid host (and also no workDir).
	sender := &Sender{
		iface: iface,
	}

	for _, tc := range testcases {
		args, err := sender.configToArgs(context.Background(), tc.c)
		if err != nil {
			t.Errorf("case %q failed with err=%v", tc.name, err)
		} else if !reflect.DeepEqual(args, tc.expect) {
			t.Errorf("case %q returns unexpected result: got %v, want %v", tc.name, args, tc.expect)
		}
	}
}

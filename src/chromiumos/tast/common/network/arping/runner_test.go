// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arping

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestParseOutput(t *testing.T) {
	for testi, testcase := range []struct {
		output   string
		expected *Result
	}{
		{
			output: `ARPING 192.168.87.20 from 192.168.87.10 eth0
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  1.159ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  1.237ms
Unicast request from 192.168.87.20 [3C:28:6D:C4:79:F9]  250.916ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  2.047ms
Unicast request from 192.168.87.20 [3C:28:6D:C4:79:F9]  251.098ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  2.301ms
Unicast request from 192.168.87.20 [3C:28:6D:C4:79:F9]  250.787ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  1.369ms
Unicast request from 192.168.87.20 [3C:28:6D:C4:79:F9]  250.556ms
Sent 5 probes (5 broadcast(s))
Received 9 response(s) (4 request(s))
`,
			expected: &Result{
				Sent:       5,
				Received:   5,
				Loss:       0,
				AvgLatency: (1159 + 1237 + 2047 + 2301 + 1369) * time.Microsecond / time.Duration(5),
				ResponderIPs: []string{
					"192.168.87.20",
					"192.168.87.20",
					"192.168.87.20",
					"192.168.87.20",
					"192.168.87.20"},
				ResponderMACs: []string{
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9"},
				Latencies: []time.Duration{
					1159 * time.Microsecond,
					1237 * time.Microsecond,
					2047 * time.Microsecond,
					2301 * time.Microsecond,
					1369 * time.Microsecond,
				},
			},
		},
		{
			output: `ARPING 192.168.87.87 from 192.168.87.10 eth0
Sent 5 probes (5 broadcast(s))
Received 0 response(s)
`,
			expected: &Result{
				Sent:       5,
				Received:   0,
				Loss:       100,
				AvgLatency: time.Duration(0),
			},
		},
		{
			output: `ARPING 192.168.87.20 from 192.168.87.10 eth0
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  1.392ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  1.678ms
Unicast reply from 192.168.87.20 [3C:28:6D:C4:79:F9]  2.138ms
Sent 5 probes (5 broadcast(s))
Received 3 response(s)
`,
			expected: &Result{
				Sent:       5,
				Received:   3,
				Loss:       40,
				AvgLatency: (1392 + 1678 + 2138) * time.Microsecond / time.Duration(3),
				ResponderIPs: []string{
					"192.168.87.20",
					"192.168.87.20",
					"192.168.87.20"},
				ResponderMACs: []string{
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9"},
				Latencies: []time.Duration{
					1392 * time.Microsecond,
					1678 * time.Microsecond,
					2138 * time.Microsecond,
				},
			},
		},
	} {
		res, err := parseOutput(testcase.output)
		if err != nil {
			t.Errorf("Case %d: %v", testi, err)
			continue
		}
		if !cmpResult(*res, *testcase.expected) {
			t.Errorf("Case %d: result not match, got %#v want %#v", testi, res, testcase.expected)
		}
	}
}

func cmpResult(r1, r2 Result) bool {
	// Round-off error may occur on Loss, so we check the differences here instead of "==" directly.
	if math.Abs(r1.Loss-r2.Loss) > 1e-6 {
		return false
	}
	r1.Loss = 0
	r2.Loss = 0

	return reflect.DeepEqual(r1, r2)
}

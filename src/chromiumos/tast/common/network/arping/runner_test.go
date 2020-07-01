// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arping

import (
	"math"
	"reflect"
	"testing"
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
				AvgLatency: (1.159 + 1.237 + 2.047 + 2.301 + 1.369) / 5,
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
				Latencies: []float64{1.159, 1.237, 2.047, 2.301, 1.369},
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
				AvgLatency: 0,
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
				AvgLatency: (1.392 + 1.678 + 2.138) / 3,
				ResponderIPs: []string{
					"192.168.87.20",
					"192.168.87.20",
					"192.168.87.20"},
				ResponderMACs: []string{
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9",
					"3C:28:6D:C4:79:F9"},
				Latencies: []float64{1.392, 1.678, 2.138},
			},
		},
	} {
		res, err := parseOutput(testcase.output)
		if err != nil {
			t.Errorf("Case %d: %v", testi, err)
			continue
		}
		if !cmpResult(*res, *testcase.expected) {
			t.Errorf("Case %d: result not match, got %+v want %+v", testi, res, testcase.expected)
		}
	}
}

func cmpResult(r1, r2 Result) bool {
	// Round-off error may occur on AvgLatency and Loss, so we check the differences here instead of "==" directly.

	if math.Abs(r1.AvgLatency-r2.AvgLatency) > 1e-6 {
		return false
	}
	r1.AvgLatency = 0
	r2.AvgLatency = 0

	if math.Abs(r1.Loss-r2.Loss) > 1e-6 {
		return false
	}
	r1.Loss = 0
	r2.Loss = 0

	return reflect.DeepEqual(r1, r2)
}

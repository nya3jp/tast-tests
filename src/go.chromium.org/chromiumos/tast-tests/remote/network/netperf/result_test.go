// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"math"
	"testing"
	"time"
)

// TestParseNetperfOutput tests parseNetperfOutput.
func TestParseNetperfOutput(t *testing.T) {
	testcases := []struct {
		testType TestType
		output   string
		errorRet bool
		result   Result
	}{
		{testType: TestTypeTCPStream,
			output: `TCP STREAM TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET to foo.bar.com (10.10.10.3) port 0 AF_INET
Recv   Send    Send
Socket Socket  Message  Elapsed
Size   Size    Size     Time     Throughput
bytes  bytes   bytes    secs.    10^6bits/sec

87380  16384  16384    10.00      941.28`,
			result: Result{
				TestType:     TestTypeTCPStream,
				Duration:     10 * time.Second,
				Measurements: map[Category]float64{CategoryThroughput: 941.28},
			},
		},
		{testType: TestTypeUDPStream,
			output: `UDP UNIDIRECTIONAL SEND TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET to foo.bar.com (10.10.10.3) port 0 AF_INET
Socket  Message  Elapsed      Messages
Size    Size     Time         Okay Errors   Throughput
bytes   bytes    secs            #      #   10^6bits/sec

129024   65507   10.00         3673      1     961.87
131072           10.00         3672            961.87`,
			result: Result{
				TestType:     TestTypeUDPStream,
				Duration:     10 * time.Second,
				Measurements: map[Category]float64{CategoryThroughput: 961.87, CategoryErrors: 2.0},
			},
		},
		{testType: TestTypeTCPCRR,
			output: `TCP REQUEST/RESPONSE TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET to foo.bar.com (10.10.10.3) port 0 AF_INET
Local /Remote
Socket Size   Request  Resp.   Elapsed  Trans.
Send   Recv   Size     Size    Time     Rate
bytes  Bytes  bytes    bytes   secs.    per sec

16384  87380  1        1       10.00     14118.53
16384  87380`,
			result: Result{
				TestType:     TestTypeTCPCRR,
				Duration:     10 * time.Second,
				Measurements: map[Category]float64{CategoryTransactionRate: 14118.53},
			},
		},
	}

	for tc, testcase := range testcases {
		ret, err := parseNetperfOutput(nil, testcase.testType, testcase.output, 10*time.Second)
		if !testcase.errorRet && (err != nil) {
			t.Errorf("tc %d:Unexpected error %v", tc, err)
		}
		if testcase.errorRet && (err == nil) {
			t.Errorf("tc %d:No error when expected", tc)
		}
		// No point in analyzing ret if error returned.
		if testcase.errorRet {
			continue
		}
		if ret.TestType != testcase.result.TestType {
			t.Errorf("tc %d:Wrong test type, returned %s, should be %s", tc, string(ret.TestType), string(testcase.result.TestType))
		}
		if ret.Duration != testcase.result.Duration {
			t.Errorf("tc %d:Wrong duration, returned %v, should be %v", tc, ret.Duration, testcase.result.Duration)
		}
		categories := []Category{CategoryThroughput, CategoryTransactionRate, CategoryErrors}
		for _, category := range categories {
			if math.Abs(ret.Measurements[category]-testcase.result.Measurements[category]) > 0.0001 {
				t.Errorf("tc %d:Mismatched results for %s: %f != %f", tc, string(category),
					ret.Measurements[category], testcase.result.Measurements[category])
			}
		}
	}
}

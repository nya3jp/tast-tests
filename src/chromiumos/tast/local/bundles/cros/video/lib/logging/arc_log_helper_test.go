// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logging

import (
	"testing"
)

func TestPassedLog(t *testing.T) {
	const log = `
[ RUN      ] ArcVideoEncoderE2ETest.TestSimpleEncode
[       OK ] ArcVideoEncoderE2ETest.TestSimpleEncode (184 ms)
[ RUN      ] ArcVideoEncoderE2ETest.TestBitrate
[       OK ] ArcVideoEncoderE2ETest.TestBitrate (768 ms)
[----------] 2 tests from ArcVideoEncoderE2ETest (953 ms total)

[----------] Global test environment tear-down
[==========] 2 tests from 1 test case ran. (954 ms total)
[  PASSED  ] 2 tests.
`
	if err := CheckARCTestResult(log); err != nil {
		t.Fatalf("Returned err should be nil but got %q", err.Error())
	}
}

func TestFailedLogSingleFailure(t *testing.T) {
	const log = `
../../../../../../../tmp/portage/media-libs/arc-codec-test-9999/work/arc-codec-test-9999/platform2/arc/codec-test/arc_video_encoder_e2e_test.cc:258: Failure
Value of: encoder_->Encode()
  Actual: true
Expected: false
[  FAILED  ] ArcVideoEncoderE2ETest.TestSimpleEncode (89 ms)
[----------] 1 test from ArcVideoEncoderE2ETest (89 ms total)

[----------] Global test environment tear-down
[==========] 1 test from 1 test case ran. (89 ms total)
[  PASSED  ] 0 tests.
[  FAILED  ] 1 test, listed below:
[  FAILED  ] ArcVideoEncoderE2ETest.TestSimpleEncode

 1 FAILED TEST
`
	if err := CheckARCTestResult(log); err == nil {
		t.Fatal("Returned err should NOT be nil")
	}
}

func TestFailedLogMultipleFailures(t *testing.T) {
	const log = `
../../../../../../../tmp/portage/media-libs/arc-codec-test-9999/work/arc-codec-test-9999/platform2/arc/codec-test/arc_video_decoder_e2e_test.cc:225: Failure
Expected: (g_env->num_frames()) != (decoded_frames_), actual: 250 vs 250
[  FAILED  ] TestSimpleDecode/ArcVideoDecoderE2EParamTest.SimpleDecode/0, where GetParam() = (false) (2839 ms)
[----------] 1 test from TestSimpleDecode/ArcVideoDecoderE2EParamTest (2840 ms total)

[----------] Global test environment tear-down
[==========] 2 tests from 2 test cases ran. (5589 ms total)
[  PASSED  ] 0 tests.
[  FAILED  ] 2 tests, listed below:
[  FAILED  ] TestSimpleDecode/ArcVideoDecoderE2EParamTest.SimpleDecode/0, where GetParam() = (false)
[  FAILED  ] TestFPS/ArcVideoDecoderE2EParamTest.SimpleDecode/0, where GetParam() = (true)

 2 FAILED TESTS
`
	if err := CheckARCTestResult(log); err == nil {
		t.Fatal("Returned err should NOT be nil")
	}
}

func TestUnfinishedLog(t *testing.T) {
	const log = `
/system/bin/sh: /data/local/tmp/arcvideodecoder_test_x86: not found
`
	if err := CheckARCTestResult(log); err == nil {
		t.Fatal("Returned err should NOT be nil")
	}
}

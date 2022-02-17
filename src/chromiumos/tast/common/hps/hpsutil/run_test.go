// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hpsutil

import (
	"testing"
)

func TestDecodePresenceResult(t *testing.T) {
	for _, tc := range []struct {
		input  uint16
		output int
	}{
		{
			input:  0x0000,
			output: 0,
		},
		{
			input:  0xff80,
			output: -128,
		},
		{
			input:  0x0080,
			output: -128,
		},
		{
			input:  0xff7f,
			output: 127,
		},
		{
			input:  0x007f,
			output: 127,
		},
		{
			input:  0xffff,
			output: -1,
		},
	} {
		result := DecodePresenceResult(tc.input)
		if result != tc.output {
			t.Errorf("For (0x%x) Expected (%d) Got (%d)", int(tc.input), tc.output, result)
		}
	}
}

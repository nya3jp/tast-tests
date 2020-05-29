// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package network provides general CrOS network goodies.
package network

import (
	"testing"
)

func TestUint16sEqualUint32s(t *testing.T) {
	testcases := []struct {
		a     []uint16
		b     []uint32
		equal bool
	}{
		{
			a:     []uint16{1, 2, 3},
			b:     []uint32{1, 2, 3},
			equal: true,
		},
		{
			a:     []uint16{3, 2, 1},
			b:     []uint32{1, 2, 3},
			equal: true,
		},
		{
			a:     []uint16{1, 2, 3},
			b:     []uint32{3, 2, 1},
			equal: true,
		},
		{
			a:     []uint16{},
			b:     []uint32{},
			equal: true,
		},
		{
			a:     []uint16{1},
			b:     []uint32{1, 2, 3},
			equal: false,
		},
		{
			a:     []uint16{1, 2, 3},
			b:     []uint32{1},
			equal: false,
		},
	}

	for i, tc := range testcases {
		equal := uint16sEqualUint32s(tc.a, tc.b)
		if equal != tc.equal {
			t.Errorf("testcase %d failed; got %t; want %t", i, equal, tc.equal)
		}
	}
}

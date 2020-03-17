// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"reflect"
	"testing"
)

func TestFilterVirtualInterface(t *testing.T) {
	inputIfaces := []string{"wlan0", "arc_wlan0", "arcbr0", "eth0", "vmtap0"}
	expectedIfaces := []string{"wlan0", "eth0"}
	outputIfaces := filterVirtualInterface(inputIfaces)
	if !reflect.DeepEqual(expectedIfaces, outputIfaces) {
		t.Fatalf("failed to test network.filterVirtualInterface(): expect result: %q, actual: %q", expectedIfaces, outputIfaces)
	}

	outputIfaces = filterVirtualInterface([]string{})
	if outputIfaces != nil {
		t.Fatalf("failed to test network.filterVirtualInterface(): expect nil, actual: %q", outputIfaces)
	}
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package debugd provides a series of tests to verify debugd's
// D-Bus API behavior.
package debugd

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/testing"
)

func testConservativeScheduler(ctx context.Context, s *testing.State, d *debugd) {
	s.Log("Running testConservativeScheduler")
	status, err := d.setSchedulerConfiguration(ctx, "conservative")
	if err != nil {
		s.Error("Failed to run SetSchedulerConfiguration: ", err)
		return
	}

	if status != true {
		s.Error("Failed to run SetSchedulerConfiguration")
		return
	}

	// Now see that CPUs are are offline.
	terminator := [1]byte{0xa}
	offlineDat, err := ioutil.ReadFile("/sys/devices/system/cpu/offline")
	if err != nil {
		s.Error("Failed to open offline cpu file: ", err)
		return
	}
	if len(offlineDat) <= 1 || offlineDat[0] == terminator[0] {
		s.Error("No offline CPUs reported: ", string(offlineDat))
		return
	}
}

func testPerformanceScheduler(ctx context.Context, s *testing.State, d *debugd) {
	s.Log("Running testPerformanceScheduler")
	status, err := d.setSchedulerConfiguration(ctx, "performance")
	if err != nil {
		s.Error("Failed to run SetSchedulerConfiguration: ", err)
		return
	}

	if status != true {
		s.Error("Failed to run SetSchedulerConfiguration")
		return
	}

	// Now see that no CPUs are offline.
	terminator := [1]byte{0xa}
	offlineDat, err := ioutil.ReadFile("/sys/devices/system/cpu/offline")
	if err != nil {
		s.Error("Failed to open offline cpu file: ", err)
		return
	}
	if len(offlineDat) != 1 || offlineDat[0] != terminator[0] {
		s.Error("Offline CPUs reported: ", string(offlineDat))
		return
	}
}

// RunTests runs a series of tests.
func RunTests(ctx context.Context, s *testing.State) {
	d, err := newDebugd(ctx)
	if err != nil {
		s.Fatal("Failed to connect debugd D-Bus service: ", err)
	}

	// Make sure the dbus calls work in various orderings.
	testPerformanceScheduler(ctx, s, d)
	testConservativeScheduler(ctx, s, d)
	testPerformanceScheduler(ctx, s, d)
	testConservativeScheduler(ctx, s, d)
	testConservativeScheduler(ctx, s, d)
	testPerformanceScheduler(ctx, s, d)
	testPerformanceScheduler(ctx, s, d)
}

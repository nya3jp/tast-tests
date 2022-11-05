// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sched contains scheduler-related ChromeOS tests
package sched

import (
	"context"
	"os"
	"time"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)
//	"chromiumos/tast/common/fixture"

func init() {
	testing.AddTest(&testing.Test{
		Func:         LatencySensitiveExists,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Ensure latency sensitive proc file is supported by kernel",
		Contacts:     []string{"joelaf@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.ARM64()),
		Timeout:      3 * time.Minute,
	})
}

// LatencySensitiveExists : Function to test whether latency_sensitive proc entry exists.
func LatencySensitiveExists(ctx context.Context, s *testing.State) {
	if _, err := os.Stat("/proc/thread-self/latency_sensitive"); err != nil {
		s.Fatal("latency_sensitive proc entry does not exist.", err)
	}
}

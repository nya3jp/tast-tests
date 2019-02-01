// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
)

type scheduler string

const (
	conservative = "conservative"
	performance  = "performance"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CoreScheduler,
		Desc:         "Verifies Debugd's SetSchedulerConfiguration D-Bus API works",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"amd64"},
	})
}

func CoreScheduler(ctx context.Context, s *testing.State) {
	d, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect debugd D-Bus service: ", err)
	}

	testScheduler := func(ctx context.Context, s *testing.State, d *debugd.Debugd, sched scheduler, expectOfflineCPUs bool) bool {
		status, err := d.SetSchedulerConfiguration(ctx, string(sched))
		if err != nil {
			s.Error("Failed to run SetSchedulerConfiguration: ", err)
			return false
		}

		if !status {
			s.Error("SetSchedulerConfiguration returned false")
			return false
		}

		// Now see if the CPUs are offline.
		terminator := [1]byte{0xa}
		offlineDat, err := ioutil.ReadFile("/sys/devices/system/cpu/offline")
		if err != nil {
			s.Error("Failed to open offline cpu file: ", err)
			return false
		}
		if expectOfflineCPUs && (len(offlineDat) <= 1 || offlineDat[0] == terminator[0]) {
			s.Error("No offline CPUs reported: ", string(offlineDat))
			return false
		}

		return true
	}

	// Restore the original setting.
	defer func(ctx context.Context) {
		if status, err := d.SetSchedulerConfiguration(ctx, string(performance)); err != nil {
			s.Error("Failed to restore device state: ", err)
		} else if !status {
			s.Error("Failed to restore device state")
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Make sure the dbus calls work in various orderings.
	if !testScheduler(ctx, s, d, performance, false) {
		return
	}

	if !testScheduler(ctx, s, d, conservative, true) {
		return
	}

	if !testScheduler(ctx, s, d, performance, false) {
		return
	}

	if !testScheduler(ctx, s, d, performance, false) {
		return
	}

	if !testScheduler(ctx, s, d, conservative, true) {
		return
	}

	if !testScheduler(ctx, s, d, conservative, true) {
		return
	}

	if !testScheduler(ctx, s, d, performance, false) {
		return
	}

	if !testScheduler(ctx, s, d, performance, false) {
		return
	}
}

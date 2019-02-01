// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package debugd

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/debugd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CoreScheduler,
		Desc: "Verifies debugd's SetSchedulerConfiguration D-Bus API works",
		Contacts: []string{
			"kerrnel@chromium.org",
			"mnissler@chromium.org",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"amd64"},
	})
}

func CoreScheduler(ctx context.Context, s *testing.State) {
	dbg, err := debugd.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to debugd D-Bus service: ", err)
	}

	testScheduler := func(sched debugd.Scheduler, expectOfflineCPUs bool) error {
		status, err := dbg.SetSchedulerConfiguration(ctx, string(sched))
		if err != nil {
			return errors.Wrap(err, "SetSchedulerConfiguration failed")
		}

		if !status {
			return errors.Wrap(err, "SetSchedulerConfiguration returned false")
		}

		// Now see if the CPUs are offline.
		offlineDat, err := ioutil.ReadFile("/sys/devices/system/cpu/offline")
		if err != nil {
			return errors.Wrap(err, "Failed to open offline CPU file")
		}
		if expectOfflineCPUs && (len(offlineDat) <= 1 || offlineDat[0] == '\n') {
			return errors.Errorf("No offline CPUs reported in %q", offlineDat)
		}
		return nil
	}

	// Restore the original setting.
	defer func(ctx context.Context) {
		if status, err := dbg.SetSchedulerConfiguration(ctx, string(debugd.Performance)); err != nil {
			s.Error("Failed to restore scheduler config: ", err)
		} else if !status {
			s.Error("Failed to restore scheduler config")
		}
	}(ctx)

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	for i, tc := range []struct {
		sched             debugd.Scheduler
		expectOfflineCPUs bool
	}{
		{debugd.Performance, false},
		{debugd.Conservative, true},
		{debugd.Performance, false},
		{debugd.Performance, false},
		{debugd.Conservative, true},
		{debugd.Conservative, true},
		{debugd.Performance, false},
		{debugd.Performance, false},
	} {
		if err := testScheduler(tc.sched, tc.expectOfflineCPUs); err != nil {
			s.Errorf("Case #%d using %s scheduler failed: %v",
				i, string(tc.sched), err)
		}
	}
}

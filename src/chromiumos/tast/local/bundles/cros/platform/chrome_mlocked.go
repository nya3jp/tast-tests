// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeMlocked,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that at least part of Chrome is mlocked",
		Contacts:     []string{"gbiv@chromium.org"},
		SoftwareDeps: []string{"chrome", "transparent_hugepage"},
		Attr:         []string{"group:mainline", "group:asan"},
	})
}

func ChromeMlocked(ctx context.Context, s *testing.State) {
	// For this test to work, some form of Chrome needs to be up and
	// running. Importantly, we must've forked zygote and the renderers.
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure that our UI is running: ", err)
	}

	// There's a race here: upstart has to create UI, which has to create
	// other processes, which have to create Chrome, which has to create
	// the Zygote. As detailed below, we can only observe mlock'ed memory
	// (currently) in zygote and its children.
	//
	// Generally, this entire process should be fast, so poll at a somewhat
	// short interval.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		hasMlocked, checkedPIDs, err := chromeHasMlockedPages(ctx)
		if err != nil {
			s.Fatal("Error checking for mlocked pages: ", err)
		}

		if !hasMlocked {
			return errors.Errorf("no mlocked pages found; checked procs %#v", checkedPIDs)
		}
		return nil
	}, &testing.PollOptions{
		Interval: 250 * time.Millisecond,
		Timeout:  30 * time.Second,
	}); err != nil {
		s.Error("Checking processes failed: ", err)
	}
}

func chromeHasMlockedPages(ctx context.Context) (hasMlocked bool, checkedPIDs []int32, err error) {
	procs, err := ashproc.Processes()
	if err != nil {
		return false, nil, errors.Wrap(err, "failed getting processes")
	}

	var pids []int32
	lockedFound := false
	for _, proc := range procs {
		rlimits, err := proc.RlimitUsage(true)
		if err != nil {
			// Ignore error because the process may be terminated.
			continue
		}

		pids = append(pids, proc.Pid)
		for _, rs := range rlimits {
			// The actual value, as long as it's non-zero, doesn't really matter here:
			// - We only try to lock a single PT_LOAD section in Chrome's binary
			// - We only do it in a subset of Chrome's processes (e.g. zygote and its children)
			// - The value is subject to change over time, as we better discover
			//   how/where to apply mlock.
			if rs.Resource == process.RLIMIT_MEMLOCK && rs.Used > 0 {
				lockedFound = true
			}
		}
	}
	return lockedFound, pids, nil
}

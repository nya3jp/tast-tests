// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeMlocked,
		Desc:         "Checks that at least part of Chrome is mlocked",
		Contacts:     []string{"gbiv@chromium.org"},
		SoftwareDeps: []string{"chrome", "transparent_hugepage"},
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
		hasMlocked, checkedPIDs, err := chromeHasMlockedPages()
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

func chromeHasMlockedPages() (hasMlocked bool, checkedPIDs []int32, err error) {
	procs, err := process.Processes()
	if err != nil {
		return false, nil, errors.Wrap(err, "failed getting processes")
	}

	lockedRegexp := regexp.MustCompile(`^VmLck:\s+(\d+)\s+kB$`)
	for _, proc := range procs {
		cmdline, err := proc.CmdlineSlice()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, nil, errors.Wrapf(err, "failed to get cmdline for %d", proc.Pid)
		}

		// Until https://crbug.com/887875 is fixed, we need to double-split. CmdlineSlice
		// will handle splitting on \0s; we need to split on spaces.
		if len(cmdline) == 0 {
			continue
		}

		cmdline = strings.Fields(cmdline[0])
		if len(cmdline) == 0 || !strings.HasSuffix(cmdline[0], "/chrome") {
			continue
		}

		// At the time of writing, proc.MemoryInfo() doesn't properly populate Locked
		// memory. Apparently other non-memory methods *do*, but they don't hand us memory
		// info... Just parse it out ourselves.
		status, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/status", proc.Pid))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, nil, errors.Wrapf(err, "failed to get status for %d", proc.Pid)
		}

		checkedPIDs = append(checkedPIDs, proc.Pid)
		foundLocked := false
		for _, line := range bytes.Split(status, []byte{'\n'}) {
			subm := lockedRegexp.FindSubmatch(line)
			if subm == nil {
				continue
			}

			foundLocked = true
			// The actual value, as long as it's non-zero, doesn't really matter here:
			// - We only try to lock a single PT_LOAD section in Chrome's binary
			// - We only do it in a subset of Chrome's processes (e.g. zygote and its children)
			// - The value is subject to change over time, as we better discover
			//   how/where to apply mlock.
			if !bytes.Equal(subm[1], []byte{'0'}) {
				return true, checkedPIDs, nil
			}
			break
		}

		// Cheap check to emit a nice diagnostic if our parsing breaks.
		if !foundLocked {
			return false, nil, errors.Errorf("no locked memory found in %d", proc.Pid)
		}
	}

	return false, checkedPIDs, nil
}

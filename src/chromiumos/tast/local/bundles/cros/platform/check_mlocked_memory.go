// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckMlockedMemory,
		Desc: "Checks that at least part of Chrome is mlocked",
		Attr: []string{"informational"},
	})
}

// Either hasMlocked == true, or we'll return checkedProcs, which indicates which PIDs we checked to
// see if we have mlocked pages.
func checkMlockedMemory() (hasMlocked bool, checkedProcs []int, err error) {
	const procDir = "/proc"

	lockedRegexp := regexp.MustCompile(`^Locked:\s+([0-9])+\s+kB$`)

	processes, err := ioutil.ReadDir(procDir)
	for _, sub := range processes {
		if !sub.IsDir() {
			continue
		}

		name := sub.Name()
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}

		pidDir := path.Join(procDir, name)
		// Looks like a PID to me.
		cmdline, err := ioutil.ReadFile(path.Join(pidDir, "cmdline"))
		if err != nil {
			// Permission errors shouldn't happen; we're root. PIDs can disappear,
			// though; we should be tolerant of that.
			if os.IsNotExist(err) {
				continue
			}
			return false, nil, errors.Wrapf(err, "getting cmdline for %d", pid)
		}

		// Some cmdlines will have \0 seps, while others will have space seps. We just want
		// to look at the executable's path, so...
		cmdline = bytes.Replace(cmdline, []byte{0}, []byte{' '}, -1)
		// If there's no data (/proc/1/cmdline), keep going without segv'ing
		if len(cmdline) == 0 {
			continue
		}

		command := bytes.Fields(cmdline)[0]
		if !bytes.HasSuffix(command, []byte("/chrome")) {
			continue
		}

		checkedProcs = append(checkedProcs, pid)
		smaps, err := ioutil.ReadFile(path.Join(pidDir, "smaps"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, nil, errors.Wrapf(err, "getting smaps for %d", pid)
		}

		foundAnyMlockedLine := false
		// The actual value, as long as it's non-zero, doesn't really matter here:
		// - We only try to lock a single PT_LOAD section in Chrome's binary
		// - We only do it in a subset of Chrome's processes (e.g. zygote and its children)
		// - Even if we did try to test the value beyond non-zeroness, the number that smaps
		//   reports is basically `vma->has_mlocked_flag() ? vma->pss : 0`. The raw value of
		//   PSS is pretty unstable, so all we can cheaply count on is that it'll be between
		//   (0, RSS) if our mlock has succeeded.
		for _, line := range bytes.Split(smaps, []byte{'\n'}) {
			subm := lockedRegexp.FindSubmatch(line)
			if subm == nil {
				continue
			}

			foundAnyMlockedLine = true
			// Note that subm[0] is the full string matched by the regexp.
			//
			if !bytes.Equal(subm[1], []byte{'0'}) {
				return true, nil, nil
			}
		}

		// Cheap attempt to catch and more accurately diagnose potential changes in the file
		// format (and/or bugs in this test).
		if !foundAnyMlockedLine {
			return false, nil, errors.Errorf("no Locked lines found for Chrome proc %d", pid)
		}
	}

	return false, checkedProcs, nil
}

func CheckMlockedMemory(ctx context.Context, s *testing.State) {
	// For this test to work, some form of Chrome needs to be up and
	// running. Importantly, we must've forked zygote and the renderers.
	if err := upstart.EnsureJobRunning(ctx, "ui"); err != nil {
		s.Fatal("Failed to ensure that our UI is running:", err)
	}

	// There's a race here: upstart has to create UI, which has to create
	// other processes, which have to create Chrome, which has to create
	// the Zygote. As detailed above, we can only observe mlock'ed memory
	// (currently) in zygote and its children.
	//
	// Generally, this entire process should be fast, so poll at a somewhat
	// short interval.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		// Any errors should be considered fatal.
		hasMlocked, checkedProcs, err := checkMlockedMemory()
		if err != nil {
			s.Fatalf("Error checking for mlocked pages: %v", err)
			return nil
		}

		if !hasMlocked {
			return errors.Errorf("no mlocked pages found; checked procs: %#v", checkedProcs)
		}
		return nil
	}, &testing.PollOptions{Interval: 250 * time.Millisecond, Timeout: 30 * time.Second})

	if err != nil {
		s.Errorf("Checking processes failed: %v", err)
	}
}

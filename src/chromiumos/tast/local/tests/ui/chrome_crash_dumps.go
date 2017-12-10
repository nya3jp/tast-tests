// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeCrashDumps,
		Desc: "Checks that Chrome writes crash dumps",
		Attr: []string{"bvt", "chrome"},
	})
}

func ChromeCrashDumps(s *testing.State) {
	const checkDumpsPollInterval = 100 * time.Millisecond

	// getAllMinidumps returns a map keyed by paths of all Chrome minidump files.
	getAllMinidumps := func() []string {
		_, minidumps, err := crash.GetCrashes(crash.ChromeCrashDir)
		if err != nil {
			s.Fatal("Failed to get minidumps: ", err)
		}
		return minidumps
	}

	// getNewMinidumps returns paths of current Chrome minidumps not present in old,
	// which should've been created via an earlier call to getAllMinidumps.
	getNewMinidumps := func(old map[string]struct{}) []string {
		nd := make([]string, 0)
		for _, p := range getAllMinidumps() {
			if _, ok := old[p]; ok {
				continue
			}
			nd = append(nd, p)
		}
		return nd
	}

	// killAndGetDumps sets SIGSEGV to the root Chrome process, waits for new minidump
	// files to be written, and then deletes them and returns their paths.
	killAndGetDumps := func() ([]string, error) {
		od := make(map[string]struct{})
		for _, p := range getAllMinidumps() {
			od[p] = struct{}{}
		}

		pid, err := chrome.GetRootPID(s.Context())
		if err != nil {
			return nil, fmt.Errorf("Failed to get root Chrome PID: %v", err)
		}
		s.Log("Sending SIGSEGV to root Chrome process ", pid)
		if err = syscall.Kill(pid, syscall.SIGSEGV); err != nil {
			return nil, err
		}

		var nd []string
		for {
			if nd = getNewMinidumps(od); len(nd) > 0 {
				break
			}
			if s.Context().Err() != nil {
				return nil, fmt.Errorf("Didn't get new minidumps: %v", s.Context().Err())
			}
			time.Sleep(checkDumpsPollInterval)
		}

		// Remove the new dumps so they don't get included in the test results.
		for _, p := range nd {
			s.Log("Deleting (expected) new minidump: ", p)
			os.Remove(p)
		}

		return nd, nil
	}

	s.Log("Restarting Chrome without logging in")
	cr, err := chrome.New(s.Context(), chrome.NoLogin())
	if err != nil {
		s.Fatal(err)
	}
	defer cr.Close(s.Context())

	if ds, err := killAndGetDumps(); err != nil {
		s.Fatal(err)
	} else if len(ds) == 0 {
		s.Error("No minidumps written to ", crash.ChromeCrashDir, " after login screen Chrome crash")
	}

	s.Log("Restarting Chrome and logging in")
	cr2, err := chrome.New(s.Context())
	if err != nil {
		s.Fatal(err)
	}
	defer cr2.Close(s.Context())

	if ds, err := killAndGetDumps(); err != nil {
		s.Fatal(err)
	} else if len(ds) == 0 {
		s.Error("No minidumps written to ", crash.ChromeCrashDir, " after logged-in Chrome crash")
	}
}

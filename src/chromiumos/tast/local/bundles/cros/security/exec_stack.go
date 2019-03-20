// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ExecStack,
		Desc: "Checks that no running processes have executable stacks",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func ExecStack(ctx context.Context, s *testing.State) {
	// Strip repeated spaces to make maps more readable.
	spaceRegexp := regexp.MustCompile("  +")

	checkMaps := func(pid int32) error {
		// Ignore errors, which likely indicate that the process went away.
		b, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
		if err != nil {
			return nil
		}

		for _, line := range strings.Split(string(b), "\n") {
			line := spaceRegexp.ReplaceAllString(strings.TrimSpace(line), " ")
			if !strings.Contains(line, "[stack") {
				continue
			}

			if parts := strings.Fields(line); len(parts) < 2 {
				return errors.Errorf("unparsable map line %q", line)
			} else if perms := parts[1]; len(perms) != 4 { // "rwxp"
				return errors.Errorf("bad perm field in %q (want e.g. \"rwxp\")", line)
			} else if !strings.Contains(perms, "w") { // sanity check
				return errors.Errorf("non-writable stack (%q)", line)
			} else if strings.Contains(perms, "x") {
				return errors.Errorf("executable stack (%q)", line)
			}
		}
		return nil
	}

	procs, err := process.Processes()
	if err != nil {
		s.Fatal("Failed to list processes: ", err)
	}
	s.Logf("Checking %v processes", len(procs))
	for _, proc := range procs {
		// Read the exe link so we can skip kernel threads.
		if exe, err := proc.Exe(); err == nil {
			if err := checkMaps(proc.Pid); err != nil {
				s.Errorf("Process %v (%v): %v", proc.Pid, exe, err)
			}
		}
	}
}

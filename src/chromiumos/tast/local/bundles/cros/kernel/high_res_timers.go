// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package kernel

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: HighResTimers,
		Desc: "Fails if timers have nanosecond resolution that is not 1 ns",
		Contacts: []string{
			"tbroch@chromium.org",
			"chromeos-kernel@google.com", // Kernel team list
			"kathrelkeld@chromium.org",   // Tast port author
		},
	})
}

// HighResTimers reads from /proc/timer_list to verify that any resolution
// listed in nsecs has a value of 1.
func HighResTimers(ctx context.Context, s *testing.State) {
	re := regexp.MustCompile(`^\s*\.resolution:\s(\d+)\s*nsecs$`)

	f, err := os.Open("/proc/timer_list")
	if err != nil {
		s.Fatal("Failed to open timer list: ", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if matches := re.FindStringSubmatch(scanner.Text()); matches != nil {
			res, err := strconv.Atoi(matches[1])
			if err != nil {
				s.Error("Error convering resolution to int: ", err)
			}
			if res != 1 {
				s.Errorf("Unexpected timer resoultion: %d ns, want 1 ns", res)
			}
		}
	}
	if scanner.Err(); err != nil {
		s.Error("Error reading timers file: ", err)
	}
}

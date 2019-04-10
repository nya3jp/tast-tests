// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Sysctl,
		Desc: "Verifies sysctl settings for ARC container",
		Contacts: []string{
			"yusukes@chromium.org", // Original author.
			"arc-eng@google.com",
			"hidehiko@chromium.org", // Tast port author.
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Timeout:      4 * time.Minute,
	})
}

func Sysctl(ctx context.Context, s *testing.State) {
	const (
		expectNonARC = 100000 // in KiB

		// The value needs to be in sync with what "/usr/share/cros/init/swap.sh get_target_value min_filelist" returns.
		expectARC = 400000 // in KiB
	)

	verify := func(expect int) {
		out, err := testexec.CommandContext(ctx, "sysctl", "-n", "vm.min_filelist_kbytes").Output(testexec.DumpLogOnError)
		if err != nil {
			s.Fatal("Failed to get vm.min_filelist_kbytes: ", err)
		}
		if val, err := strconv.Atoi(strings.TrimSpace(string(out))); err != nil {
			s.Fatal("Failed to parse sysctl output: ", err)
		} else if val != expect {
			s.Fatalf("Unexpected vm.min_filelist_kbytes: got %d; want %d", val, expect)
		}
	}

	// Restart UI to ensure ARC is once stopped.
	// Note that ARC mini container may be running.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui: ", err)
	}

	verify(expectNonARC)

	// Log in to start ARC full container.
	func() {
		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to launch Chrome: ", err)
		}
		defer cr.Close(ctx)

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to launch ARC: ", err)
		}
		defer a.Close()

		verify(expectARC)
	}()

	// Restart UI to shut down ARC once.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui: ", err)
	}

	verify(expectNonARC)
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros tests lacros-chrome running on ChromeOS.
package lacros

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/lacros/faillog"
	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Basic,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Fixture:      "lacros",
		Timeout:      7 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"lacros_stable"},
		}, {
			Name:              "unstable",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros_unstable"},
		}},
	})
}

func Basic(ctx context.Context, s *testing.State) {
	// Temporarily save the lacros-chrome's execution stats.
	// We found several "EPERM" errors on execution.
	// TODO(crbug.com/1198252): Should be removed when the issue is fixed.
	defer func() {
		// Runs the given command with redirecting its stdout to outpath.
		// On error, logging it then ignored.
		run := func(outpath string, cmds ...string) {
			out, err := os.Create(outpath)
			if err != nil {
				testing.ContextLogf(ctx, "Failed to create %q: %v", outpath, err)
				return
			}
			defer out.Close()
			cmd := testexec.CommandContext(ctx, cmds[0], cmds[1:]...)
			cmd.Stdout = out
			if err := cmd.Run(testexec.DumpLogOnError); err != nil {
				testing.ContextLogf(ctx, "Failed to run %q: %v", cmds[0], err)
			}
		}

		// Dump current mount status. Specifically noexec on /mnt/stateful_partition
		// is interesting.
		run(filepath.Join(s.OutDir(), "mount.txt"), "mount")

		// Also check the dearchived files.
		run(filepath.Join(s.OutDir(), "lacros-ls.txt"),
			"ls", "-l", s.FixtValue().(launcher.FixtData).LacrosPath)
	}()

	l, err := launcher.LaunchLacrosChrome(ctx, s.FixtValue().(launcher.FixtData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer func() {
		l.Close(ctx)
		if err := faillog.Save(s.HasError, l, s.OutDir()); err != nil {
			s.Log("Failed to save lacros logs: ", err)
		}
	}()

	if _, err = l.Devsess.CreateTarget(ctx, "about:blank"); err != nil {
		s.Fatal("Failed to open new tab: ", err)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WaylandClientTest,
		Desc: "Verifies that wayland client tests run properly",
		Contacts: []string{
			"berlu@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      time.Minute,
	})
}

// WaylandClientTest runs wayland client tests via the command line.
func WaylandClientTest(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Stopping ui service to run wayland_client_version_binding_test")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Shorten the test timeout so that even if the test timesout, there is still time to make sure ui service is running.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Execute the test binary.
	const exec = "wayland_client_tests"
	const execPath = filepath.Join(chrome.BinTestDir, exec)
	list, err := gtest.ListTests(ctx, execPath)
	if err != nil {
		s.Fatal("Failed to list gtest: ", err)
	}
	for _, testcase := range list {
		if report, err := gtest.New(
			filepath.Join(chrome.BinTestDir, exec),
			gtest.Logfile(filepath.Join(s.OutDir(), exec+".log")),
			gtest.Filter(testcase),
		).Run(shortCtx); err != nil {
			s.Errorf("Failed to run %v: %v", exec, err)
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Error(name, " failed")
				}
			}
		}
	}
}

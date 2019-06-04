// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package video

import (
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome/bintest"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"context"
	"time"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         InteractiveUiTests,
		Desc:         "Runs Chrome interactive_ui_tests to measure performance",
		Contacts:     []string{"oshima@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{},
		Data:         []string{},
	})
}

// InteractiveUiTests runs Chrome's interactive_ui_tests.
func InteractiveUiTests(ctx context.Context, s *testing.State) {
	// Reserve time to restart the ui job at the end of the test.
	// Only a single process can have access to the GPU, so we are required
	// to call "stop ui" at the start of the test. This will shut down the
	// chrome process and allow us to claim ownership of the GPU.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()
	upstart.StopJob(shortCtx, "ui")
	defer upstart.EnsureJobRunning(ctx, "ui")

	args := []string{
		"--test-launcher-jobs=1",
		"--enable-pixel-output-in-tests",
		"--dbus-stub",
		"--gtest_filter=ScreenRotation*",
	}
	env := []string{"CR_SOURCE_ROOT=/tmp", "LD_LIBRARY_PATH=/opt/google/chrome:/usr/local/lib64"}

	const exec = "interactive_ui_tests"
	if ts, err := bintest.RunWithEnv(shortCtx, exec, args, s.OutDir(), env); err != nil {
		s.Errorf("Failed to run %v: %v", exec, err)
		for _, t := range ts {
			s.Error(t, " failed")
		}
	}
}

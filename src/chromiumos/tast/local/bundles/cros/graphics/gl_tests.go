// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GLTests,
		Desc: "Verifies gl_tests runs successfully",
		Contacts: []string{
			"dcastagna@chromium.org",
			"chromeos-gfx@google.com",
		},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

func GLTests(ctx context.Context, s *testing.State) {
	testing.ContextLog(ctx, "Stopping ui service to run gl_tests")
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui service: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	exe := "/usr/local/libexec/chrome-binary-tests/gl_tests"
	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := testexec.CommandContext(ctx, exe)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Error("Failed to run gl_tests: ", err)
	}
}

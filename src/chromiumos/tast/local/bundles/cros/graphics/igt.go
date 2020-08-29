// Copyright 2020 The Chromium OS Authors. All rights reserved.
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

// igtTest is used to describe the config used to run each test.
type igtTest struct {
	exe string // The test executable name.
}

func init() {
	testing.AddTest(&testing.Test{
		Func: IGT,
		Desc: "Verifies igt-gpu-tools test binaries run successfully",
		Contacts: []string{
			"ddavenport@chromium.org",
			"chromeos-gfx@google.com",
		},
		Params: []testing.Param{{
			Name: "kms_atomic",
			Val: igtTest{
				exe: "kms_atomic",
			},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "kms_addfb_basic",
			Val: igtTest{
				exe: "kms_addfb_basic",
			},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			Timeout:           5 * time.Minute,
		}, {
			Name: "kms_plane",
			Val: igtTest{
				exe: "kms_plane",
			},
			ExtraSoftwareDeps: []string{"drm_atomic"},
			Timeout:           5 * time.Minute,
		}},
		Attr: []string{"group:graphics", "graphics_perbuild"},
	})
}

func IGT(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui job: ", err)
	}

	testOpt := s.Param().(igtTest)

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	exePath := filepath.Join("/usr/local/libexec/igt-gpu-tools", testOpt.exe)
	cmd := testexec.CommandContext(ctx, exePath)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exePath, err)
	}
}

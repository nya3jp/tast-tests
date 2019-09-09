// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

type optImpl struct {
	command string
	timeout time.Duration
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DRM,
		Desc: "Verifies DRM-related test binaries run successfully",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Params: []testing.Param{{
			Name:              "drm_cursor_test",
			Val:               optImpl{command: "drm_cursor_test"},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "linear_bo_test",
			Val:               optImpl{command: "linear_bo_test"},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "null_platform_test",
			Val:               optImpl{command: "null_platform_test"},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "swrast_test",
			Val:       optImpl{command: "swrast_test"},
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "atomic_test",
			Val:               optImpl{command: "atomictest -a -t all", timeout: 5 * time.Minute},
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
		}, {
			Name:              "mmap_test",
			Val:               optImpl{command: "mmap_test", timeout: 5 * time.Minute},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
		}, {
			Name:              "vgem_test",
			Val:               optImpl{command: "vgem_test"},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
		}, {
			Name:              "vk_glow",
			Val:               optImpl{command: "vk_glow"},
			ExtraSoftwareDeps: []string{"display_backlight", "vulkan"},
			ExtraAttr:         []string{"group:crosbolt", "crosbolt_nightly"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func DRM(ctx context.Context, s *testing.State) {
	if err := setUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer tearDown(ctx)

	testOpt := s.Param().(optImpl)
	timeout := 20 * time.Second
	if testOpt.timeout > timeout {
		timeout = testOpt.timeout
	}
	splitCommand := strings.Fields(testOpt.command)
	cmd := "/usr/local/bin/" + splitCommand[0]
	runTest(ctx, s, timeout, cmd, splitCommand[1:]...)
}

// setUp prepares the testing environment to run RunTest().
func setUp(ctx context.Context) error {
	testing.ContextLog(ctx, "Setting up DRM test")
	return upstart.StopJob(ctx, "ui")
}

// tearDown restores the working environment after RunTest().
func tearDown(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down DRM test")
	return upstart.EnsureJobRunning(ctx, "ui")
}

// runTest runs the exe binary test.
// Before running, SetUp() must be called.
func runTest(ctx context.Context, s *testing.State, t time.Duration, exe string, args ...string) {
	s.Log("Running ", shutil.EscapeSlice(append([]string{exe}, args...)))

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exe, err)
	}
}

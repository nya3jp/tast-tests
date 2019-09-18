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
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// drmTest is used to describe the config used to run each drm_test.
type drmTest struct {
	command []string      // The command path to be run. This should be relative to /usr/local/bin.
	timeout time.Duration // Timeout to run the drmTest.
}

func init() {
	var (
		atomic       = drmTest{command: []string{"atomictest", "-a", "-t", "all"}, timeout: 5 * time.Minute}
		drmCursor    = drmTest{command: []string{"drm_cursor_test"}, timeout: 20 * time.Second}
		linearBo     = drmTest{command: []string{"linear_bo_test"}, timeout: 20 * time.Second}
		mmap         = drmTest{command: []string{"mmap_test"}, timeout: 5 * time.Minute}
		nullPlatform = drmTest{command: []string{"null_platform_test"}, timeout: 20 * time.Second}
		swrast       = drmTest{command: []string{"swrast_test"}, timeout: 20 * time.Second}
		vgem         = drmTest{command: []string{"vgem_test"}, timeout: 20 * time.Second}
		vkGlow       = drmTest{command: []string{"vk_glow"}, timeout: 20 * time.Second}
	)

	testing.AddTest(&testing.Test{
		Func: DRM,
		Desc: "Verifies DRM-related test binaries run successfully",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Params: []testing.Param{{
			Name: "bvt",
			Val: []drmTest{
				drmCursor,
				linearBo,
				nullPlatform,
				vgem},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test",
			Val:               []drmTest{atomic},
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "mmap_test",
			Val:               []drmTest{mmap},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "swrast_test",
			Val:       []drmTest{swrast},
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "vk_glow",
			Val:               []drmTest{vkGlow},
			ExtraSoftwareDeps: []string{"display_backlight", "vulkan"},
			ExtraAttr:         []string{"informational"},
		}},
		Timeout: 5 * time.Minute,
	})
}

func DRM(ctx context.Context, s *testing.State) {
	if err := setUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer tearDown(ctx)

	testOpts := s.Param().([]drmTest)
	for _, testOpt := range testOpts {
		runTest(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
	}
}

// setUp prepares the testing environment to run runTest().
func setUp(ctx context.Context) error {
	testing.ContextLog(ctx, "Setting up DRM test")
	return upstart.StopJob(ctx, "ui")
}

// tearDown restores the working environment after runTest().
func tearDown(ctx context.Context) error {
	testing.ContextLog(ctx, "Tearing down DRM test")
	return upstart.EnsureJobRunning(ctx, "ui")
}

// runTest runs the exe binary test. This method may be called several times as long as setUp() has been invoked beforehand.
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

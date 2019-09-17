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

type option struct {
	command []string
	timeout time.Duration
	runDeps []string
}

func init() {
	allTestOpts := map[string]option{
		"atomic_test":        {command: []string{"atomictest", "-a", "-t", "all"}, timeout: 5 * time.Minute},
		"drm_cursor_test":    {command: []string{"drm_cursor_test"}, timeout: 20 * time.Second},
		"linear_bo_test":     {command: []string{"linear_bo_test"}, timeout: 20 * time.Second},
		"mmap_test":          {command: []string{"mmap_test"}, timeout: 5 * time.Minute},
		"null_platform_test": {command: []string{"null_platform_test"}, timeout: 20 * time.Second},
		"swrast_test":        {command: []string{"swrast_test"}, timeout: 20 * time.Second},
		"vgem_test":          {command: []string{"vgem_test"}, timeout: 20 * time.Second},
		"vk_glow":            {command: []string{"vk_glow"}, timeout: 20 * time.Second},
	}

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
			Val: []option{
				allTestOpts["drm_cursor_test"],
				allTestOpts["linear_bo_test"],
				allTestOpts["null_platform_test"],
				allTestOpts["vgem_test"]},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test",
			Val:               []option{allTestOpts["atomic_test"]},
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "mmap_test",
			Val:               []option{allTestOpts["mmap_test"]},
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:      "swrast_test",
			Val:       []option{allTestOpts["swrast_test"]},
			ExtraAttr: []string{"informational"},
		}, {
			Name:              "vk_glow",
			Val:               []option{allTestOpts["vk_glow"]},
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

	testOpts := s.Param().([]option)
	for _, testOpt := range testOpts {
		cmd := "/usr/local/bin/" + testOpt.command[0]
		runTest(ctx, s, testOpt.timeout, cmd, testOpt.command[1:]...)
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

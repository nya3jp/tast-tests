// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/autocaps"
	"chromiumos/tast/local/bundles/cros/graphics/drm"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// testCategory refers to the type of test to be run.
type testCategory string

const (
	binary   testCategory = "Binary"   // The test type is a binary executable.
	function testCategory = "Function" // The test type is a Go function.
)

// drmTest is used to describe the config used to run each drm_test.
type drmTest struct {
	// Test category, e.g. binary or function.
	category testCategory
	// The command path to be run, if this is a binary test, or the function to
	// call. If it's a binary exe, it should be relative to /usr/local/bin.
	command []string
	// Timeout to run the drmTest.
	timeout time.Duration
}

// Map of function names to actual functions in this file.
var functions = map[string]func(*testing.State){
	"verifyDRMAtomic": verifyDRMAtomic,
}

func init() {
	testing.AddTest(&testing.Test{
		Func: DRM,
		Desc: "Verifies DRM-related functionality run successfully",
		Contacts: []string{
			"andrescj@chromium.org",
			"chromeos-gfx@google.com",
			"hidehiko@chromium.org", // Tast port.
		},
		Params: []testing.Param{{
			Name:              "atomic_test",
			Val:               drmTest{category: binary, command: []string{"atomictest", "-a", "-t", "all"}, timeout: 5 * time.Minute},
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
		}, {
			Name:              "drm_cursor_test",
			Val:               drmTest{category: binary, command: []string{"drm_cursor_test"}, timeout: 20 * time.Second},
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "linear_bo_test",
			Val:               drmTest{category: binary, command: []string{"linear_bo_test"}, timeout: 20 * time.Second},
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "mmap_test",
			Val:               drmTest{category: binary, command: []string{"mmap_test"}, timeout: 5 * time.Minute},
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "null_platform_test",
			Val:               drmTest{category: binary, command: []string{"null_platform_test"}, timeout: 20 * time.Second},
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name: "swrast_test",
			Val:  drmTest{category: binary, command: []string{"swrast_test"}, timeout: 20 * time.Second},
		}, {
			Name:              "vgem_test",
			Val:               drmTest{category: binary, command: []string{"vgem_test"}, timeout: 20 * time.Second},
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "vk_glow",
			Val:               drmTest{category: binary, command: []string{"vk_glow"}, timeout: 20 * time.Second},
			ExtraSoftwareDeps: []string{"display_backlight", "vulkan"},
		}, {
			Name: "verify_drm_atomic",
			Val:  drmTest{category: function, command: []string{"verifyDRMAtomic"}, timeout: 20 * time.Second},
		}},
		Attr:    []string{"group:mainline", "informational"},
		Timeout: 5 * time.Minute,
	})
}

func DRM(ctx context.Context, s *testing.State) {
	if err := setUp(ctx); err != nil {
		s.Fatal("Failed to set up the DRM test: ", err)
	}
	defer tearDown(ctx)

	testOpt := s.Param().(drmTest)
	if testOpt.category == binary {
		runTest(ctx, s, testOpt.timeout, testOpt.command[0], testOpt.command[1:]...)
	} else {
		functions[testOpt.command[0]](s)
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

// verifyDRMAtomic verifies that the "drm_atomic" capability presence or absence
// corelates with the support on the actual DRM API.
func verifyDRMAtomic(s *testing.State) {
	// Get capabilities computed by autocaps package and extract drm_atomic.
	staticCaps, err := autocaps.Read(autocaps.DefaultCapabilityDir, nil)
	isDrmAtomicEnabled := staticCaps["drm_atomic"] == autocaps.Yes

	supported, err := drm.IsDRMAtomicSupported()
	if err != nil {
		s.Fatal("Failed to verify drm atomic support: ", err)
	}
	if supported != isDrmAtomicEnabled {
		if supported {
			s.Fatal("DRM atomic should NOT be supported but it is")
		} else {
			s.Fatal("DRM atomic should be supported but it is NOT")
		}
	}
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

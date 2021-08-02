// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

// igtTest is used to describe the config used to run each test.
type igtTest struct {
	exe string // The test executable name.
}

// resultSummary is a summary of results from an igt test log.
type resultSummary struct {
	passed  int // number of passed subtests
	failed  int // number of failed subtests
	skipped int // number of skipped subtests
}

var subtestResultRegex = regexp.MustCompile("^Subtest .*: ([A-Z]+)")

func init() {
	testing.AddTest(&testing.Test{
		Func: IGT,
		Desc: "Verifies igt-gpu-tools test binaries run successfully",
		Contacts: []string{
			"ddavenport@chromium.org",
			"chromeos-gfx@google.com",
			"chromeos-gfx-display@google.com",
			"markyacoub@google.com",
		},
		SoftwareDeps: []string{"drm_atomic", "igt", "no_qemu"},
		Params: []testing.Param{{
			Name: "drm_import_export",
			Val: igtTest{
				exe: "drm_import_export",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "drm_mm",
			Val: igtTest{
				exe: "drm_mm",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "drm_read",
			Val: igtTest{
				exe: "drm_read",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_addfb_basic",
			Val: igtTest{
				exe: "kms_addfb_basic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_atomic",
			Val: igtTest{
				exe: "kms_atomic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_atomic_interruptible",
			Val: igtTest{
				exe: "kms_atomic_interruptible",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_atomic_transition",
			Val: igtTest{
				exe: "kms_atomic_transition",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_big_fb",
			Val: igtTest{
				exe: "kms_big_fb",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_busy",
			Val: igtTest{
				exe: "kms_busy",
			},
			Timeout: 15 * time.Minute,
		}, {
			Name: "kms_color",
			Val: igtTest{
				exe: "kms_color",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_concurrent",
			Val: igtTest{
				exe: "kms_concurrent",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_content_protection",
			Val: igtTest{
				exe: "kms_content_protection",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_crtc_background_color",
			Val: igtTest{
				exe: "kms_crtc_background_color",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_cursor_crc",
			Val: igtTest{
				exe: "kms_cursor_crc",
			},
			Timeout: 15 * time.Minute,
		}, {
			Name: "kms_cursor_legacy",
			Val: igtTest{
				exe: "kms_cursor_legacy",
			},
			Timeout: 20 * time.Minute,
		}, {
			Name: "kms_dp_aux_dev",
			Val: igtTest{
				exe: "kms_dp_aux_dev",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_dp_dsc",
			Val: igtTest{
				exe: "kms_dp_dsc",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_draw_crc",
			Val: igtTest{
				exe: "kms_draw_crc",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_flip",
			Val: igtTest{
				exe: "kms_flip",
			},
			Timeout: 30 * time.Minute,
		}, {
			Name: "kms_flip_event_leak",
			Val: igtTest{
				exe: "kms_flip_event_leak",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_frontbuffer_tracking",
			Val: igtTest{
				exe: "kms_frontbuffer_tracking",
			},
			Timeout: 25 * time.Minute,
		}, {
			Name: "kms_getfb",
			Val: igtTest{
				exe: "kms_getfb",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_hdmi_inject",
			Val: igtTest{
				exe: "kms_hdmi_inject",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_hdr",
			Val: igtTest{
				exe: "kms_hdr",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_invalid_dotclock",
			Val: igtTest{
				exe: "kms_invalid_dotclock",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_multipipe_modeset",
			Val: igtTest{
				exe: "kms_multipipe_modeset",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_panel_fitting",
			Val: igtTest{
				exe: "kms_panel_fitting",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_pipe_crc_basic",
			Val: igtTest{
				exe: "kms_pipe_crc_basic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane",
			Val: igtTest{
				exe: "kms_plane",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane_alpha_blend",
			Val: igtTest{
				exe: "kms_plane_alpha_blend",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane_cursor",
			Val: igtTest{
				exe: "kms_plane_cursor",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane_lowres",
			Val: igtTest{
				exe: "kms_plane_lowres",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane_multiple",
			Val: igtTest{
				exe: "kms_plane_multiple",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_plane_scaling",
			Val: igtTest{
				exe: "kms_plane_scaling",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_prime",
			Val: igtTest{
				exe: "kms_prime",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_prop_blob",
			Val: igtTest{
				exe: "kms_prop_blob",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_properties",
			Val: igtTest{
				exe: "kms_properties",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_psr",
			Val: igtTest{
				exe: "kms_psr",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_psr2_su",
			Val: igtTest{
				exe: "kms_psr2_su",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_rmfb",
			Val: igtTest{
				exe: "kms_rmfb",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_rotation_crc",
			Val: igtTest{
				exe: "kms_rotation_crc",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_selftest",
			Val: igtTest{
				exe: "kms_selftest",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_setmode",
			Val: igtTest{
				exe: "kms_setmode",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_sysfs_edid_timing",
			Val: igtTest{
				exe: "kms_sysfs_edid_timing",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_universal_plane",
			Val: igtTest{
				exe: "kms_universal_plane",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_vblank",
			Val: igtTest{
				exe: "kms_vblank",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "kms_vrr",
			Val: igtTest{
				exe: "kms_vrr",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "sw_sync",
			Val: igtTest{
				exe: "sw_sync",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "testdisplay",
			Val: igtTest{
				exe: "testdisplay",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "vgem_basic",
			Val: igtTest{
				exe: "vgem_basic",
			},
			Timeout: 5 * time.Minute,
		}, {
			Name: "vgem_slow",
			Val: igtTest{
				exe: "vgem_slow",
			},
			Timeout: 5 * time.Minute,
		}},
		Attr:    []string{"group:graphics", "graphics_perbuild"},
		Fixture: "graphicsNoChrome",
	})
}

func summarizeLog(f *os.File) resultSummary {
	var r resultSummary
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := subtestResultRegex.FindStringSubmatch(scanner.Text()); m != nil {
			switch m[1] {
			case "SKIP":
				r.skipped++
			case "FAIL":
				r.failed++
			case "SUCCESS":
				r.passed++
			}
		}
	}
	return r
}

func IGT(ctx context.Context, s *testing.State) {
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
	err = cmd.Run()
	exitErr, isExitErr := err.(*exec.ExitError)

	// Reset the file to the beginning so the log can be read out again.
	f.Seek(0, 0)
	results := summarizeLog(f)
	summary := fmt.Sprintf("Ran %d subtests with %d failures and %d skipped",
		results.passed+results.failed, results.failed, results.skipped)

	if results.passed+results.failed+results.skipped == 0 {
		// TODO(markyacoub): Many tests have igt_require_intel(), which automatically skips
		// everything on other platforms. Mark the test as PASS for now until there are no more
		// platform specific dependencies
		s.Log("Entire test was skipped - No subtests were run")
		// In the case of running multiple subtests which all happen to be skipped, igt_exitcode is 0,
		// but the final exit code will be 77.
	} else if results.passed+results.failed == 0 && isExitErr && exitErr.ExitCode() == 77 {
		s.Log("____________________________________________________")
		s.Logf("ALL %d subtests were SKIPPED: %s", results.skipped, err.Error())
		s.Log("----------------------------------------------------")
	} else if err != nil {
		s.Errorf("Error running %s: %v", exePath, err)
		s.Error(summary)
	} else {
		s.Log(summary)
	}
}

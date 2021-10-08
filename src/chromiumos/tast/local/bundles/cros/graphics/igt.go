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
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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

var subtestResultRegex = regexp.MustCompile("^Subtest (.*): ([A-Z]+)")

var gpuAmd = []string{"zork", "grunt"}
var gpuQcom = []string{"strongbad", "trogdor"}
var gpuMtk = []string{"kukui", "jacuzzi"}

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
		Attr:         []string{"group:graphics", "graphics_igt"},
		Fixture:      "chromeGraphicsIgt",
		Params: []testing.Param{{
			Name: "drm_import_export",
			Val: igtTest{
				exe: "drm_import_export",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "drm_mm",
			Val: igtTest{
				exe: "drm_mm",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "drm_read",
			Val: igtTest{
				exe: "drm_read",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_addfb_basic",
			Val: igtTest{
				exe: "kms_addfb_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_atomic",
			Val: igtTest{
				exe: "kms_atomic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_perbuild"},
		}, {
			Name: "kms_atomic_interruptible",
			Val: igtTest{
				exe: "kms_atomic_interruptible",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_perbuild"},
		}, {
			Name: "kms_atomic_transition",
			Val: igtTest{
				exe: "kms_atomic_transition",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_atomic_transition_unstable",
			Val: igtTest{
				exe: "kms_atomic_transition",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_color",
			Val: igtTest{
				exe: "kms_color",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_color_unstable",
			Val: igtTest{
				exe: "kms_color",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_concurrent",
			Val: igtTest{
				exe: "kms_concurrent",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_concurrent_unstable",
			Val: igtTest{
				exe: "kms_concurrent",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_content_protection",
			Val: igtTest{
				exe: "kms_content_protection",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_cursor_crc",
			Val: igtTest{
				exe: "kms_cursor_crc",
			},
			Timeout:           15 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...)),
		}, {
			Name: "kms_cursor_crc_unstable",
			Val: igtTest{
				exe: "kms_cursor_crc",
			},
			Timeout:           15 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...)),
		}, {
			Name: "kms_cursor_legacy",
			Val: igtTest{
				exe: "kms_cursor_legacy",
			},
			Timeout:           20 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_cursor_legacy_unstable",
			Val: igtTest{
				exe: "kms_cursor_legacy",
			},
			Timeout:           20 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_dp_aux_dev",
			Val: igtTest{
				exe: "kms_dp_aux_dev",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_dp_dsc",
			Val: igtTest{
				exe: "kms_dp_dsc",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_flip",
			Val: igtTest{
				exe: "kms_flip",
			},
			Timeout:           30 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...), hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_flip_unstable",
			Val: igtTest{
				exe: "kms_flip",
			},
			Timeout:           30 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...), hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_flip_event_leak",
			Val: igtTest{
				exe: "kms_flip_event_leak",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_getfb",
			Val: igtTest{
				exe: "kms_getfb",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_hdmi_inject",
			Val: igtTest{
				exe: "kms_hdmi_inject",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_hdr",
			Val: igtTest{
				exe: "kms_hdr",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_multipipe_modeset",
			Val: igtTest{
				exe: "kms_multipipe_modeset",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_panel_fitting",
			Val: igtTest{
				exe: "kms_panel_fitting",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_panel_fitting_unstable",
			Val: igtTest{
				exe: "kms_panel_fitting",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_pipe_crc_basic",
			Val: igtTest{
				exe: "kms_pipe_crc_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_plane",
			Val: igtTest{
				exe: "kms_plane",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_plane_unstable",
			Val: igtTest{
				exe: "kms_plane",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_plane_alpha_blend",
			Val: igtTest{
				exe: "kms_plane_alpha_blend",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_plane_alpha_blend_unstable",
			Val: igtTest{
				exe: "kms_plane_alpha_blend",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_plane_cursor",
			Val: igtTest{
				exe: "kms_plane_cursor",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...), hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_plane_cursor_unstable",
			Val: igtTest{
				exe: "kms_plane_cursor",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...), hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_plane_lowres",
			Val: igtTest{
				exe: "kms_plane_lowres",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_plane_multiple",
			Val: igtTest{
				exe: "kms_plane_multiple",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_plane_multiple_unstable",
			Val: igtTest{
				exe: "kms_plane_multiple",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_plane_scaling",
			Val: igtTest{
				exe: "kms_plane_scaling",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...), hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_plane_scaling_unstable",
			Val: igtTest{
				exe: "kms_plane_scaling",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...), hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_prime",
			Val: igtTest{
				exe: "kms_prime",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...)),
		}, {
			Name: "kms_prime_unstable",
			Val: igtTest{
				exe: "kms_prime",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...)),
		}, {
			Name: "kms_prop_blob",
			Val: igtTest{
				exe: "kms_prop_blob",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_properties",
			Val: igtTest{
				exe: "kms_properties",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_properties_unstable",
			Val: igtTest{
				exe: "kms_properties",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_rmfb",
			Val: igtTest{
				exe: "kms_rmfb",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_rotation_crc",
			Val: igtTest{
				exe: "kms_rotation_crc",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_selftest",
			Val: igtTest{
				exe: "kms_selftest",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_setmode",
			Val: igtTest{
				exe: "kms_setmode",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_sysfs_edid_timing",
			Val: igtTest{
				exe: "kms_sysfs_edid_timing",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_universal_plane",
			Val: igtTest{
				exe: "kms_universal_plane",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_vblank",
			Val: igtTest{
				exe: "kms_vblank",
			},
			Timeout:   15 * time.Minute,
			ExtraAttr: []string{"graphics_weekly"},
		}, {
			Name: "kms_vrr",
			Val: igtTest{
				exe: "kms_vrr",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "sw_sync",
			Val: igtTest{
				exe: "sw_sync",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "testdisplay",
			Val: igtTest{
				exe: "testdisplay",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "vgem_basic",
			Val: igtTest{
				exe: "vgem_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "vgem_slow",
			Val: igtTest{
				exe: "vgem_slow",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "template",
			Val: igtTest{
				exe: "template",
			},
			Timeout:   1 * time.Minute,
			ExtraAttr: []string{"group:mainline"},
		}},
	})
}

func summarizeLog(f *os.File) (r resultSummary, failedSubtests []string) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := subtestResultRegex.FindStringSubmatch(scanner.Text()); m != nil {
			subtestName := m[1]
			result := m[2]
			switch result {
			case "SKIP":
				r.skipped++
			case "FAIL":
				r.failed++
				failedSubtests = append(failedSubtests, subtestName)
			case "SUCCESS":
				r.passed++
			}
		}
	}
	return r, failedSubtests
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
	results, failedSubtests := summarizeLog(f)
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
	} else if len(failedSubtests) > 0 {
		s.Logf("Error running %s: %v", exePath, err)
		s.Log(summary)
		s.Log("Failed subtests: ", failedSubtests)
		failedSubtestsMessage := ""
		if len(failedSubtests) <= 3 {
			failedSubtestsMessage = strings.Join(failedSubtests, " ")
		} else {
			failedSubtestsMessage = failedSubtests[0] + " ... " + failedSubtests[len(failedSubtests)-1]
		}
		s.Errorf("%s Pass:%d Fail:%d (%s)", testOpt.exe, results.passed, results.failed, failedSubtestsMessage)
	} else {
		s.Log(summary)
	}
}

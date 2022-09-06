// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var subtestResultRegex = regexp.MustCompile("^Subtest (.*): ([A-Z]+)")

var gpuAmd = []string{"zork", "grunt"}
var gpuQcom = []string{"strongbad", "trogdor"}
var gpuMtk = []string{"kukui", "jacuzzi"}

func init() {
	testing.AddTest(&testing.Test{
		Func: IgtKms,
		Desc: "Verifies IGT KMS test binaries run successfully",
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
			Val: graphics.IgtTest{
				Exe: "drm_import_export",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "drm_mm",
			Val: graphics.IgtTest{
				Exe: "drm_mm",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "drm_read",
			Val: graphics.IgtTest{
				Exe: "drm_read",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_addfb_basic",
			Val: graphics.IgtTest{
				Exe: "kms_addfb_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_atomic",
			Val: graphics.IgtTest{
				Exe: "kms_atomic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_perbuild"},
		}, {
			Name: "kms_atomic_interruptible",
			Val: graphics.IgtTest{
				Exe: "kms_atomic_interruptible",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_perbuild"},
		}, {
			Name: "kms_atomic_transition",
			Val: graphics.IgtTest{
				Exe: "kms_atomic_transition",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuQcom...)),
		}, {
			Name: "kms_atomic_transition_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_atomic_transition",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuQcom...)),
		}, {
			Name: "kms_color",
			Val: graphics.IgtTest{
				Exe: "kms_color",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_color_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_color",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_concurrent",
			Val: graphics.IgtTest{
				Exe: "kms_concurrent",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_concurrent_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_concurrent",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_content_protection",
			Val: graphics.IgtTest{
				Exe: "kms_content_protection",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_cursor_crc",
			Val: graphics.IgtTest{
				Exe: "kms_cursor_crc",
			},
			Timeout:           15 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...)),
		}, {
			Name: "kms_cursor_crc_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_cursor_crc",
			},
			Timeout:           15 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...)),
		}, {
			Name: "kms_cursor_legacy",
			Val: graphics.IgtTest{
				Exe: "kms_cursor_legacy",
			},
			Timeout:           20 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_cursor_legacy_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_cursor_legacy",
			},
			Timeout:           20 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_dp_aux_dev",
			Val: graphics.IgtTest{
				Exe: "kms_dp_aux_dev",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_dp_dsc",
			Val: graphics.IgtTest{
				Exe: "kms_dp_dsc",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_flip",
			Val: graphics.IgtTest{
				Exe: "kms_flip",
			},
			Timeout:           30 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(append(gpuAmd, gpuQcom...), gpuMtk...)...)),
		}, {
			Name: "kms_flip_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_flip",
			},
			Timeout:           30 * time.Minute,
			ExtraAttr:         []string{"graphics_weekly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(append(gpuAmd, gpuQcom...), gpuMtk...)...)),
		}, {
			Name: "kms_flip_event_leak",
			Val: graphics.IgtTest{
				Exe: "kms_flip_event_leak",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_getfb",
			Val: graphics.IgtTest{
				Exe: "kms_getfb",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_hdmi_inject",
			Val: graphics.IgtTest{
				Exe: "kms_hdmi_inject",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_hdr",
			Val: graphics.IgtTest{
				Exe: "kms_hdr",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_multipipe_modeset",
			Val: graphics.IgtTest{
				Exe: "kms_multipipe_modeset",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_panel_fitting",
			Val: graphics.IgtTest{
				Exe: "kms_panel_fitting",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_panel_fitting_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_panel_fitting",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_pipe_crc_basic",
			Val: graphics.IgtTest{
				Exe: "kms_pipe_crc_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_plane",
			Val: graphics.IgtTest{
				Exe: "kms_plane",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_plane",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_alpha_blend",
			Val: graphics.IgtTest{
				Exe: "kms_plane_alpha_blend",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_alpha_blend_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_plane_alpha_blend",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_cursor",
			Val: graphics.IgtTest{
				Exe: "kms_plane_cursor",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(append(gpuAmd, gpuQcom...), gpuMtk...)...)),
		}, {
			Name: "kms_plane_cursor_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_plane_cursor",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(append(gpuAmd, gpuQcom...), gpuMtk...)...)),
		}, {
			Name: "kms_plane_lowres",
			Val: graphics.IgtTest{
				Exe: "kms_plane_lowres",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_plane_multiple",
			Val: graphics.IgtTest{
				Exe: "kms_plane_multiple",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_multiple_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_plane_multiple",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_scaling",
			Val: graphics.IgtTest{
				Exe: "kms_plane_scaling",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_plane_scaling_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_plane_scaling",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(append(gpuAmd, gpuQcom...)...)),
		}, {
			Name: "kms_prime",
			Val: graphics.IgtTest{
				Exe: "kms_prime",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuAmd...)),
		}, {
			Name: "kms_prime_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_prime",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuAmd...)),
		}, {
			Name: "kms_prop_blob",
			Val: graphics.IgtTest{
				Exe: "kms_prop_blob",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_properties",
			Val: graphics.IgtTest{
				Exe: "kms_properties",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform(gpuMtk...)),
		}, {
			Name: "kms_properties_unstable",
			Val: graphics.IgtTest{
				Exe: "kms_properties",
			},
			Timeout:           5 * time.Minute,
			ExtraAttr:         []string{"graphics_nightly"},
			ExtraHardwareDeps: hwdep.D(hwdep.Platform(gpuMtk...)),
		}, {
			Name: "kms_rmfb",
			Val: graphics.IgtTest{
				Exe: "kms_rmfb",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_rotation_crc",
			Val: graphics.IgtTest{
				Exe: "kms_rotation_crc",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_selftest",
			Val: graphics.IgtTest{
				Exe: "kms_selftest",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_setmode",
			Val: graphics.IgtTest{
				Exe: "kms_setmode",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_sysfs_edid_timing",
			Val: graphics.IgtTest{
				Exe: "kms_sysfs_edid_timing",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_universal_plane",
			Val: graphics.IgtTest{
				Exe: "kms_universal_plane",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "kms_vblank",
			Val: graphics.IgtTest{
				Exe: "kms_vblank",
			},
			Timeout:   15 * time.Minute,
			ExtraAttr: []string{"graphics_weekly"},
		}, {
			Name: "kms_vrr",
			Val: graphics.IgtTest{
				Exe: "kms_vrr",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "sw_sync",
			Val: graphics.IgtTest{
				Exe: "sw_sync",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "testdisplay",
			Val: graphics.IgtTest{
				Exe: "testdisplay",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "vgem_basic",
			Val: graphics.IgtTest{
				Exe: "vgem_basic",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "vgem_slow",
			Val: graphics.IgtTest{
				Exe: "vgem_slow",
			},
			Timeout:   5 * time.Minute,
			ExtraAttr: []string{"graphics_nightly"},
		}, {
			Name: "template",
			Val: graphics.IgtTest{
				Exe: "template",
			},
			Timeout:   1 * time.Minute,
			ExtraAttr: []string{"group:mainline"},
		}},
	})
}

func IgtKms(ctx context.Context, s *testing.State) {
	testOpt := s.Param().(graphics.IgtTest)
	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(testOpt.Exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	isExitErr, exitErr, err := graphics.IgtExecuteTests(ctx, testOpt.Exe, f)

	isError, outputLog := graphics.IgtProcessResults(testOpt.Exe, f, isExitErr, exitErr, err)

	if isError {
		s.Error(outputLog)
	} else {
		s.Log(outputLog)
	}
}

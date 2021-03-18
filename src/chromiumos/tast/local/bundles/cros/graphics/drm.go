// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

var drmResultRegex = regexp.MustCompile("ERROR:")

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
			Name:              "atomic_test_crtc_background_color",
			Val:               []string{"atomictest", "-a", "-t", "crtc_background_color"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_crtc_ctm",
			Val:               []string{"atomictest", "-a", "-t", "crtc_ctm"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_crtc_gamma",
			Val:               []string{"atomictest", "-a", "-t", "crtc_gamma"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_disable_primary",
			Val:               []string{"atomictest", "-a", "-t", "disable_primary"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_fullscreen_video",
			Val:               []string{"atomictest", "-a", "-t", "fullscreen_video"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_in_fence",
			Val:               []string{"atomictest", "-a", "-t", "in_fence"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_multiple_planes",
			Val:               []string{"atomictest", "-a", "-t", "multiple_planes"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_orientation",
			Val:               []string{"atomictest", "-a", "-t", "orientation"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_out_fence",
			Val:               []string{"atomictest", "-a", "-t", "out_fence"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_overlay_downscaling",
			Val:               []string{"atomictest", "-a", "-t", "overlay_downscaling"},
			Timeout:           9 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_overlay_pageflip",
			Val:               []string{"atomictest", "-a", "-t", "overlay_pageflip"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_overlay_upscaling",
			Val:               []string{"atomictest", "-a", "-t", "overlay_upscaling"},
			Timeout:           6 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_plane_alpha",
			Val:               []string{"atomictest", "-a", "-t", "plane_alpha"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_plane_ctm",
			Val:               []string{"atomictest", "-a", "-t", "plane_ctm"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_primary_pageflip",
			Val:               []string{"atomictest", "-a", "-t", "primary_pageflip"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_rgba_primary",
			Val:               []string{"atomictest", "-a", "-t", "rgba_primary"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_video_overlay",
			Val:               []string{"atomictest", "-a", "-t", "video_overlay"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "atomic_test_video_underlay",
			Val:               []string{"atomictest", "-a", "-t", "video_underlay"},
			Timeout:           1 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight", "drm_atomic"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "drm_cursor_test",
			Val:               []string{"drm_cursor_test"},
			Timeout:           30 * time.Second,
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "dmabuf_test",
			Val:               []string{"dmabuf_test"},
			Timeout:           30 * time.Second,
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnPlatform("coral", "pyro", "reef", "sand", "snappy")),
			ExtraSoftwareDeps: []string{"display_backlight"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "linear_bo_test",
			Val:               []string{"linear_bo_test"},
			Timeout:           30 * time.Second,
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "mmap_test",
			Val:               []string{"mmap_test"},
			Timeout:           15 * time.Minute,
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:              "null_platform_test",
			Val:               []string{"null_platform_test"},
			Timeout:           30 * time.Second,
			ExtraSoftwareDeps: []string{"display_backlight"},
		}, {
			Name:    "swrast_test",
			Val:     []string{"swrast_test"},
			Timeout: 30 * time.Second,
		}, {
			Name:              "vk_glow",
			Val:               []string{"vk_glow"},
			Timeout:           30 * time.Second,
			ExtraSoftwareDeps: []string{"display_backlight", "vulkan"},
			ExtraAttr:         []string{"informational"},
		}},
		Attr:    []string{"group:mainline"},
		Fixture: "gpuWatchHangs",
	})
}

// DRM runs DRM/KMS related test via the command line.
func DRM(ctx context.Context, s *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Shorten the test timeout so that even if the test timesout, there is still time to make sure ui service is running.
	shortCtx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	commands := s.Param().([]string)
	runTest(shortCtx, s, commands[0], commands[1:]...)
}

func getErrorLog(f *os.File) string {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if drmResultRegex.FindString(scanner.Text()) != "" {
			return scanner.Text()
		}
	}
	return ""
}

// runTest runs the exe binary test and records the output into a file.
func runTest(ctx context.Context, s *testing.State, exe string, args ...string) {
	s.Log("Running ", shutil.EscapeSlice(append([]string{exe}, args...)))

	f, err := os.Create(filepath.Join(s.OutDir(), filepath.Base(exe)+".txt"))
	if err != nil {
		s.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	cmd := testexec.CommandContext(ctx, exe, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Errorf("Failed to run %s: %v", exe, err)
	}

	f.Seek(0, 0)
	results := getErrorLog(f)
	if results != "" {
		s.Log(results)
	}
}

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// overlay defines the parameters for a HW overlay (a.k.a. DRM plane).
type overlay struct {
	format string
	size   string
}

// overlaysTestParam defines the overlays structure for a test case.
type overlaysTestParam struct {
	primaryFormats []string
	overlay        overlay
}

func init() {
	testing.AddTest(&testing.Test{
		Func: PlatformOverlays,
		Desc: "Checks that certain configurations of primary and overlay planes are indeed supported",
		Contacts: []string{
			"mcasas@chromium.org",
			"chromeos-gfx-compositor@google.com",
		},
		Attr:         []string{"group:graphics", "graphics_perbuild"},
		SoftwareDeps: []string{"video_overlays", "no_qemu"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Timeout:      time.Minute,
		Params: []testing.Param{{
			Name: "24bpp",
			Val: overlaysTestParam{
				primaryFormats: []string{"XR24", "XB24", "AR24", "AB24"},
			},
		}, {
			Name: "30bpp",
			Val: overlaysTestParam{
				primaryFormats: []string{"AR30", "AB30", "XR30", "XB30"},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Supports30bppFramebuffer()),
		}, {
			Name: "24bpp_nv12_overlay",
			Val: overlaysTestParam{
				primaryFormats: []string{"XR24", "XB24", "AR24", "AB24"},
				overlay:        overlay{"NV12", "640x360"},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.SupportsNV12Overlays()),
		}, {
			Name: "30bpp_nv12_overlay",
			Val: overlaysTestParam{
				primaryFormats: []string{"AR30", "AB30", "XR30", "XB30"},
				overlay:        overlay{"NV12", "640x360"},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Supports30bppFramebuffer(), hwdep.SupportsNV12Overlays()),
		}, {
			Name: "24bpp_p010_overlay",
			Val: overlaysTestParam{
				primaryFormats: []string{"XR24", "XB24", "AR24", "AB24"},
				overlay:        overlay{"P010", "640x360"},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Supports30bppFramebuffer(), hwdep.SupportsNV12Overlays()),
		}, {
			Name: "30bpp_p010_overlay",
			Val: overlaysTestParam{
				primaryFormats: []string{"AR30", "AB30", "XR30", "XB30"},
				overlay:        overlay{"P010", "640x360"},
			},
			ExtraHardwareDeps: hwdep.D(hwdep.Supports30bppFramebuffer(), hwdep.SupportsNV12Overlays()),
		}},
		Fixture: "gpuWatchHangs",
	})
}

// PlatformOverlays runs plane_test binary test for a given format.
func PlatformOverlays(ctx context.Context, state *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		state.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	const testCommand string = "plane_test"

	f, err := os.Create(filepath.Join(state.OutDir(), filepath.Base(testCommand)+".txt"))
	if err != nil {
		state.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	const formatFlag string = "--format"
	primaryFormats := state.Param().(overlaysTestParam).primaryFormats
	overlayFormat := state.Param().(overlaysTestParam).overlay.format
	overlaySize := state.Param().(overlaysTestParam).overlay.size
	invocationError := make(map[string]error)

	for _, primaryFormat := range primaryFormats {
		params := []string{formatFlag, primaryFormat}
		if overlayFormat != "" {
			params = append(params, "--plane", formatFlag, overlayFormat, "--size", overlaySize)
		}

		invocationCommand := shutil.EscapeSlice(append([]string{testCommand}, params...))
		state.Log("Running ", invocationCommand)

		cmd := testexec.CommandContext(ctx, testCommand, params...)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			invocationError[invocationCommand] = err
		} else {
			// TODO(b/217970618): Parse the DRM response or debugfs to verify that the
			// actual plane combination ris what was intended.
			state.Logf("Run succeeded for %s", invocationCommand)
			// Same as Chrome, any one of the primaryFormats needs to be supported.
			return
		}
	}

	state.Errorf("Failed to run %s for all formats", testCommand)
	for command, err := range invocationError {
		exitCode, ok := testexec.ExitCode(err)
		if !ok {
			state.Errorf("Failed to run %s: %v", command, err)
		} else {
			state.Errorf("Command %s exited with status %v", command, exitCode)
		}
	}
}

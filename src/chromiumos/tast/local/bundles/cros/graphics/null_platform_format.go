// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/graphics"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: NullPlatformFormat,
		// TODO: update description
		Desc: "Runs the null_platform_test for a given format",
		Contacts: []string{
			"clarissagarvey@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		// TODO: should this run perbuild?
		Attr: []string{"group:graphics", "graphics_perbuild"},
		// TODO: deps list? drm.go has display_backlight but I don't know why
		// SoftwareDeps: []string{""},
		Timeout: time.Minute,
		Params: []testing.Param{{
			Name: "24bpp",
			Val:  []string{"-f", "XR24"},
		}},
	})
}

// NullPlatformFormat runs null_platform_test binary test for a given format.
func NullPlatformFormat(ctx context.Context, state *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		state.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	// Get supported formats
	formats, err := graphics.ModetestPrimaryDisplayFormatsSupported(ctx)
	if err != nil {
		state.Fatal("Failed to get formats: ", err)
	} else {
		// TODO: better logging here
		state.Log("Found formats:")
		for _, format := range formats {
			state.Log(format)
		}
	}
	defer graphics.DumpModetestOnError(ctx, state.OutDir(), state.HasError)

	// Try all formats. null_platform_test will fail with an error
	// message about unsupported format if that format does not exist.
	// TODO: actually do the above. Currently still running -f XR24
	//       (This would have a function factored out; not all in this function directly)
	const testCommand string = "null_platform_test"
	formatArgs := state.Param().([]string)

	state.Log("Running ", shutil.EscapeSlice(append([]string{testCommand}, formatArgs...)))

	f, err := os.Create(filepath.Join(state.OutDir(), filepath.Base(testCommand)+".txt"))
	if err != nil {
		state.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	// Execute the null_format test for a given format
	cmd := testexec.CommandContext(ctx, testCommand, formatArgs...)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		state.Errorf("Failed to run %s: %v", testCommand, err)
	}
}

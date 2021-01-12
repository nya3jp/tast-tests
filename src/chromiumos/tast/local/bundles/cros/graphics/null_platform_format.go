// Copyright 2021 The Chromium OS Authors. All rights reserved.
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

func init() {
	testing.AddTest(&testing.Test{
		Func: NullPlatformFormat,
		Desc: "Checks that the null_platform_test passes for at least one formats with a given color depth",
		Contacts: []string{
			"clarissagarvey@chromium.org",
			"chromeos-gfx-video@google.com",
		},
		Attr:    []string{"group:graphics", "graphics_perbuild"},
		Timeout: time.Minute,
		Params: []testing.Param{{
			Name: "24bpp",
			Val:  []string{"AR24", "AB24", "XR24", "XB24"},
		}},
	})
}

// NullPlatformFormat runs null_platform_test binary test for a given format.
func NullPlatformFormat(ctx context.Context, state *testing.State) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		state.Fatal("Failed to stop ui job: ", err)
	}
	defer upstart.EnsureJobRunning(ctx, "ui")

	const testCommand string = "null_platform_test"
	const formatFlag string = "-f"
	formats := state.Param().([]string)
	invocationError := make(map[string]error)

	f, err := os.Create(filepath.Join(state.OutDir(), filepath.Base(testCommand)+".txt"))
	if err != nil {
		state.Fatal("Failed to create a log file: ", err)
	}
	defer f.Close()

	for _, format := range formats {
		invocationCommand := shutil.EscapeSlice([]string{testCommand, formatFlag, format})
		state.Log("Running ", invocationCommand)

		// Execute the null_platform_test for a given format
		cmd := testexec.CommandContext(ctx, testCommand, []string{formatFlag, format}...)
		cmd.Stdout = f
		cmd.Stderr = f
		if err := cmd.Run(); err != nil {
			invocationError[invocationCommand] = err
		} else {
			state.Logf("Run succeeded for %s", invocationCommand)
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

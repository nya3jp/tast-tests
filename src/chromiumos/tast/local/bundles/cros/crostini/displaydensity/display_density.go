// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package displaydensity

import (
	"context"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

// RunTest executes a X11 or wayland test application directly from the command
// line in the terminal twice with default and low display density respectively
// and verifies that it renders the window bigger in low display density.
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont *vm.Container, conf crostini.DemoConfig) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	type density int

	const (
		lowDensity density = iota
		highDensity
	)

	demoWindowSize := func(densityConfiguration density) (crostini.Size, error) {
		windowName := conf.Name
		subCommandArgs := []string{}
		if densityConfiguration == lowDensity {
			windowName = windowName + "_low_density"
			// TODO(hollingum): Find a better way to pass environment vars to a container command (rather than invoking sh).
			subCommandArgs = append(subCommandArgs, "DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}")
		}
		subCommandArgs = append(subCommandArgs, conf.AppPath, "--width=100", "--height=100", "--title="+windowName)

		cmd := cont.Command(ctx, "sh", "-c", strings.Join(subCommandArgs, " "))
		s.Logf("Running %q", shutil.EscapeSlice(cmd.Args))
		if err := cmd.Start(); err != nil {
			return crostini.Size{}, err
		}
		defer cmd.Wait(testexec.DumpLogOnError)
		defer cmd.Kill()

		var sz crostini.Size
		var err error
		if sz, err = crostini.PollWindowSize(ctx, tconn, windowName); err != nil {
			return crostini.Size{}, err
		}
		s.Logf("Window %q size is %v", windowName, sz)

		s.Logf("Closing %q with keypress", windowName)
		err = keyboard.Accel(ctx, "Enter")

		return sz, err
	}

	sizeHighDensity, err := demoWindowSize(highDensity)
	if err != nil {
		s.Fatal("Failed getting high-density window size: ", err)
	}

	sizeLowDensity, err := demoWindowSize(lowDensity)
	if err != nil {
		s.Fatal("Failed getting low-density window size: ", err)
	}

	if err := crostini.VerifyWindowDensities(ctx, tconn, sizeHighDensity, sizeLowDensity); err != nil {
		s.Fatal("Failed during window density comparison: ", err)
	}
}

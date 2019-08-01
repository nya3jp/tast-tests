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
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	commandWidth  = "--width=100"
	commandHeight = "--height=100"
)

// RunTest executes a X11 or wayland test application directly from the command
// line in the terminal twice with default and low display density respectively
// and verifies that it renders the window bigger in low display density.
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont *vm.Container, conf *crostini.DemoConfig) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	getSizeOfDemoWindow := func(isLowDensity bool) (sz crostini.Size, err error) {
		windowName := conf.Name
		subCommandArgs := []string{}
		if isLowDensity {
			windowName = windowName + "_low_density"
			subCommandArgs = append(subCommandArgs, "DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}")
		}
		subCommandArgs = append(subCommandArgs, conf.AppPath, commandWidth, commandHeight, "--title="+windowName)

		cmd := cont.Command(ctx, "sh", "-c", strings.Join(subCommandArgs, " "))
		s.Logf("Running %q", strings.Join(cmd.Args, " "))
		if err = cmd.Start(); err != nil {
			s.Errorf("Failed launching %q: %v", strings.Join(cmd.Args, " "), err)
			cmd.DumpLog(ctx)
			return sz, err
		}
		defer cmd.Wait()
		defer cmd.Kill()

		if sz, err = crostini.GetWindowSizeWithPoll(ctx, tconn, windowName); err != nil {
			s.Errorf("Failed getting window %q size: %v", windowName, err)
			return sz, err
		}
		s.Logf("Window %q size is %v", windowName, sz)

		s.Logf("Closing %q with keypress", windowName)
		err = keyboard.Accel(ctx, "Enter")

		return sz, err
	}

	sizeHighDensity, err := getSizeOfDemoWindow(false)
	if err != nil {
		s.Error("Failed to get the high-density size: ", err)
		return
	}

	sizeLowDensity, err := getSizeOfDemoWindow(true)
	if err != nil {
		s.Error("Failed to get the low-density size: ", err)
		return
	}

	if sizeHighDensity.W > sizeLowDensity.W || sizeHighDensity.H > sizeLowDensity.H {
		s.Errorf("App %q has high density size %v greater than low density size %v", conf.Name, sizeHighDensity, sizeLowDensity)
		return
	}

	tabletMode, err := crostini.IsTabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Error("Failed getting tablet mode: ", err)
		return
	}
	s.Log("Tablet mode is ", tabletMode)

	factor, err := crostini.GetPrimaryDisplayScaleFactor(ctx, tconn)
	if err != nil {
		s.Error("Failed getting primary display scale factor: ", err)
		return
	}
	s.Log("Primary display scale factor is ", factor)

	if factor != 1.0 && !tabletMode && (sizeHighDensity.W == sizeLowDensity.W || sizeHighDensity.H == sizeLowDensity.H) {
		s.Errorf("App %q has high density and low density windows with the same size of %v while the scale factor is %v", conf.Name, sizeHighDensity, factor)
		return
	}
}

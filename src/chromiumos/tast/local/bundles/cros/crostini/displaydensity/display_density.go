// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package displaydensity

import (
	"context"
	"fmt"
	"strings"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest executes a X11 or wayland test application directly from the command
// line in the terminal twice with default and low display density respectively
// and verifies that it renders the window bigger in low display density.
func RunTest(ctx context.Context, s *testing.State, tconn *chrome.Conn, cont *vm.Container, name, command string) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	commandWidth := fmt.Sprintf("--width=%d", 100)
	commandHeight := fmt.Sprintf("--height=%d", 100)
	commandTitle := "--title=" + name
	cmd := cont.Command(ctx, command, commandWidth, commandHeight, commandTitle)
	s.Logf("Running %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", strings.Join(cmd.Args, " "), err)
		cmd.DumpLog(ctx)
		return
	}

	sizeHighDensity, err := crostini.GetWindowSizeWithPoll(ctx, tconn, name)

	if err != nil {
		s.Errorf("Failed getting window %q size: %v", name, err)
		return
	}
	s.Logf("Window %q size is %v", name, sizeHighDensity)

	s.Logf("Closing %v with keypress", name)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	cmd.Kill()
	cmd.Wait()

	lowDensityName := name + "_low_density"
	commandTitle = "--title=" + lowDensityName
	subCommandArgs := []string{"DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}", command, commandWidth, commandHeight, commandTitle}
	subCommand := strings.Join(subCommandArgs, " ")
	cmd = cont.Command(ctx, "sh", "-c", subCommand)
	s.Logf("Running %q", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", strings.Join(cmd.Args, " "), err)
		cmd.DumpLog(ctx)
		return
	}

	sizeLowDensity, err := crostini.GetWindowSizeWithPoll(ctx, tconn, lowDensityName)

	if err != nil {
		s.Errorf("Failed getting window %q size: %v", lowDensityName, err)
		return
	}
	s.Logf("Window %q size is %v", lowDensityName, sizeLowDensity)

	s.Logf("Closing %v with keypress", lowDensityName)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	cmd.Kill()
	cmd.Wait()

	if sizeHighDensity.W > sizeLowDensity.W || sizeHighDensity.H > sizeLowDensity.H {
		s.Errorf("App %q has high density size %v greater than low density size %v", name, sizeHighDensity, sizeLowDensity)
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
		s.Errorf("App %q has high density and low density windows with the same size of %v while the scale factor is %v", name, sizeHighDensity, factor)
		return
	}
}

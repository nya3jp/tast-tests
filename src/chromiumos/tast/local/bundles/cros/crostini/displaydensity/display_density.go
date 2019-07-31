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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const (
	commandWidth  = "--width=100"
	commandHeight = "--height=100"
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

	// TODO(hollingum): Factor common code from the high/low dpi executions into a function.
	commandWidth := fmt.Sprintf("--width=%d", 100)
	commandHeight := fmt.Sprintf("--height=%d", 100)
	commandTitle := "--title=" + name
	highDensityCmd := cont.Command(ctx, command, commandWidth, commandHeight, commandTitle)
	s.Logf("Running %q", shutil.EscapeSlice(highDensityCmd.Args))
	if err := highDensityCmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", shutil.EscapeSlice(highDensityCmd.Args), err)
		return
	}
	defer highDensityCmd.Wait(testexec.DumpLogOnError)
	defer highDensityCmd.Kill()

	sizeHighDensity, err := crostini.PollWindowSize(ctx, tconn, name)

	if err != nil {
		s.Fatalf("Failed getting window %q size: %v", name, err)
	}
	s.Logf("Window %q size is %v", name, sizeHighDensity)

	s.Logf("Closing %v with keypress", name)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	lowDensityName := name + "_low_density"
	commandTitle = "--title=" + lowDensityName
	// TODO(hollingum): Find a bettwe way to pass environment vars to a container command (rather than invoking sh).
	subCommandArgs := []string{"DISPLAY=${DISPLAY_LOW_DENSITY}", "WAYLAND_DISPLAY=${WAYLAND_DISPLAY_LOW_DENSITY}", command, commandWidth, commandHeight, commandTitle}
	subCommand := strings.Join(subCommandArgs, " ")
	lowDensityCmd := cont.Command(ctx, "sh", "-c", subCommand)
	s.Logf("Running %q", shutil.EscapeSlice(lowDensityCmd.Args))
	if err := lowDensityCmd.Start(); err != nil {
		s.Errorf("Failed launching %q: %v", shutil.EscapeSlice(lowDensityCmd.Args), err)
		return
	}
	defer lowDensityCmd.Wait(testexec.DumpLogOnError)
	defer lowDensityCmd.Kill()

	sizeLowDensity, err := crostini.PollWindowSize(ctx, tconn, lowDensityName)

	if err != nil {
		s.Fatalf("Failed getting window %q size: %v", lowDensityName, err)
	}
	s.Logf("Window %q size is %v", lowDensityName, sizeLowDensity)

	s.Logf("Closing %v with keypress", lowDensityName)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	if sizeHighDensity.W > sizeLowDensity.W || sizeHighDensity.H > sizeLowDensity.H {
		s.Fatalf("App %q has high density size %v greater than low density size %v", name, sizeHighDensity, sizeLowDensity)
	}

	tabletMode, err := crostini.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed getting tablet mode: ", err)
	}
	s.Log("Tablet mode is ", tabletMode)

	factor, err := crostini.PrimaryDisplayScaleFactor(ctx, tconn)
	if err != nil {
		s.Fatal("Failed getting primary display scale factor: ", err)
	}
	s.Log("Primary display scale factor is ", factor)

	if factor != 1.0 && !tabletMode && (sizeHighDensity.W == sizeLowDensity.W || sizeHighDensity.H == sizeLowDensity.H) {
		s.Fatalf("App %q has high density and low density windows with the same size of %v while the scale factor is %v", name, sizeHighDensity, factor)
	}
}

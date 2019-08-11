// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package verifyapp

import (
	"context"
	"fmt"
	"image/color"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest executes a test application directly from the command
// line in the terminal and verifies that it renders the majority of pixels on
// the screen in the specified color.
func RunTest(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	cont *vm.Container, name, command string, expectedColor color.Color) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	nrgba := color.NRGBAModel.Convert(expectedColor).(color.NRGBA)
	commandColor := fmt.Sprintf("--bgcolor=0x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	commandTitle := "--title=" + name + "_terminal"
	cmd := cont.Command(ctx, command, commandColor, commandTitle)
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed launching %v: %v", command, err)
	}
	defer cmd.Wait(testexec.DumpLogOnError)
	defer cmd.Kill()

	if err := crostini.PollScreenshotDominantColor(ctx, s, cr, expectedColor, name); err != nil {
		s.Fatalf("Failed to see screenshot %q: %v", name, err)
	}

	// Terminate the app now so that if there's a failure in the
	// screenshot then we can get its output which may give us useful information
	// about display errors.
	s.Logf("Closing %v with keypress", name)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}
}

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package verifyapp

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"path/filepath"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest executes a test application directly from the command
// line in the terminal and verifies that it renders the majority of pixels on
// the screen in the specified color.
func RunTest(ctx context.Context, s *testing.State, cr *chrome.Chrome, cont *vm.Container, conf crostini.DemoConfig) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	nrgba := color.NRGBAModel.Convert(conf.DominantColor).(color.NRGBA)
	commandColor := fmt.Sprintf("--bgcolor=0x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	commandTitle := fmt.Sprintf("--title=%s_terminal", conf.Name)

	args := []string{conf.AppPath, commandColor, commandTitle}
	if conf.Width > 0 {
		args = append(args, fmt.Sprintf("--width=%v", conf.Width))
	}
	if conf.Height > 0 {
		args = append(args, fmt.Sprintf("--height=%v", conf.Height))
	}

	pre := s.PreValue().(crostini.PreData)
	tconn := pre.TestAPIConn

	scale, err := crostini.PrimaryDisplayScaleFactor(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get primary display scale factor: ", err)
	}
	// When |rect| is empty, the whole screenshot will be checked for dominant
	// color. This matches the way our apps behave when --width or --height are
	// unset.
	w, h := int(float64(conf.Width)/scale), int(float64(conf.Height)/scale)
	rect := image.Rect(0, 0, w, h)

	cmd := cont.Command(ctx, args...)
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed launching %v: %v", conf.AppPath, err)
	}
	defer cmd.Wait(testexec.DumpLogOnError)
	defer cmd.Kill()

	if err := crostini.MatchScreenshotDominantColor(ctx, cr, conf.DominantColor, filepath.Join(s.OutDir(), conf.Name+"_screenshot.png"), rect); err != nil {
		s.Fatalf("Failed to see screenshot %q: %v", conf.Name, err)
	}

	// Terminate the app now so that if there's a failure in the
	// screenshot then we can get its output which may give us useful information
	// about display errors.
	s.Logf("Closing %v with keypress", conf.Name)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}
}

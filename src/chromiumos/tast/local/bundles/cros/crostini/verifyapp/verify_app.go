// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package verifyapp

import (
	"context"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest executes a test application directly from the command
// line in the terminal and verifies that it renders the majority of pixels on
// the screen in the specified color.
func RunTest(ctx context.Context, s *testing.State, cr *chrome.Chrome, cont *vm.Container, conf *crostini.DemoConfig) {
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	nrgba := color.NRGBAModel.Convert(conf.DominantColor).(color.NRGBA)
	commandColor := fmt.Sprintf("--bgcolor=0x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	commandTitle := "--title=" + conf.Name + "_terminal"
	cmd := cont.Command(ctx, conf.AppPath, commandColor, commandTitle)
	if err := cmd.Start(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failed launching %v: %v", conf.AppPath, err)
		return
	}
	defer cmd.Wait()
	defer cmd.Kill()

	screenshotName := "screenshot_terminal_" + conf.Name + ".png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x1

	// Allow up to 10 seconds for the target screen to render.
	err = testing.Poll(ctx, func(ctx context.Context) error {
		if err := screenshot.CaptureChrome(ctx, cr, path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			s.Fatalf("Failed opening the screenshot image %v: %v", path, err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			s.Fatalf("Failed decoding the screenshot image %v: %v", path, err)
		}
		color, ratio := colorcmp.DominantColor(im)
		if ratio >= 0.5 && colorcmp.ColorsMatch(color, conf.DominantColor, maxKnownColorDiff) {
			return nil
		}
		return errors.Errorf("screenshot did not have matching dominant color, expected %v but got %v at ratio %0.2f",
			colorcmp.ColorStr(conf.DominantColor), colorcmp.ColorStr(color), ratio)
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	if err != nil {
		s.Errorf("Failure in screenshot comparison for %v from terminal: %v", conf.Name, err)
	}

	// Terminate the app now so that if there's a failure in the
	// screenshot then we can get its output which may give us useful information
	// about display errors.
	s.Logf("Closing %v with keypress", conf.Name)
	if err := keyboard.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}
}

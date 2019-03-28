// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// VerifyAppFromTerminal executes a test application directly from the command
// line in the terminal and verifies that it renders the majority of pixels on
// the screen in the specified color.
func VerifyAppFromTerminal(ctx context.Context, s *testing.State, cr *chrome.Chrome,
	cont *vm.Container, ew *input.KeyboardEventWriter, name, command string, expectedColor color.Color) {
	s.Log("Executing test app from terminal launch for ", name)
	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	nrgba := color.NRGBAModel.Convert(expectedColor).(color.NRGBA)
	commandColor := fmt.Sprintf("--bgcolor=0x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	commandTitle := "--title=" + name + "_terminal"
	cmd := cont.Command(ctx, command, commandColor, commandTitle)
	if err := cmd.Start(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failed launching %v: %v", command, err)
		return
	}

	screenshotName := "screenshot_terminal_" + name + ".png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x1

	// Allow up to 10 seconds for the target screen to render.
	err := testing.Poll(ctx, func(ctx context.Context) error {
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
		if ratio >= 0.5 && colorcmp.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		}
		return errors.Errorf("screenshot did not have matching dominant color, expected %v but got %v at ratio %0.2f",
			colorcmp.ColorStr(expectedColor), colorcmp.ColorStr(color), ratio)
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	// Terminate the app now so that if there's a failure in the
	// screenshot then we can get its output which may give us useful information
	// about display errors.
	s.Logf("Closing %v with keypress", name)
	if err := ew.Accel(ctx, "Enter"); err != nil {
		s.Error("Failed to type Enter key: ", err)
	}

	cmd.Kill()
	cmd.Wait()
	if err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failure in screenshot comparison for %v from terminal: %v", name, err)
	}
}

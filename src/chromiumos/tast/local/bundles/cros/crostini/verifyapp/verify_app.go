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
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// RunTest executes a test application and verifies that it renders the majority of pixels on
// the screen in the specified color. This test is parameterized by:
//  - A configuration for the demo application that should be run.
//  - A flag for whether the app should be run via the launcher or direcrly from the terminal.
func RunTest(ctx context.Context, s *testing.State, cr *chrome.Chrome, cont *vm.Container, conf *crostini.DemoConfig, useLauncher bool) {

	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	nrgba := color.NRGBAModel.Convert(conf.DominantColor).(color.NRGBA)
	commandColor := fmt.Sprintf("--bgcolor=0x%02x%02x%02x", nrgba.R, nrgba.G, nrgba.B)
	commandTitle := "--title=" + conf.Name + "_terminal"
	cmd := cont.Command(ctx, conf.AppPath, commandColor, commandTitle)
	if err := cmd.Start(); err != nil {
		s.Fatalf("Failed launching %v: %v", conf.AppPath, err)
	}
	defer cmd.Wait(testexec.DumpLogOnError)
	defer cmd.Kill()

	verifyScreenshot(ctx, s, cr, conf.DominantColor)

	crostini.CloseDemoWithKeyboard(ctx, s)
}

// verifyScreenshot takes a screenshot and then checks that the majority of the
// pixels in it match the passed in expected color.
func verifyScreenshot(ctx context.Context, s *testing.State, cr *chrome.Chrome, expectedColor color.Color) {
	path := filepath.Join(s.OutDir(), "screenshot.png")

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x1

	// Allow up to 10 seconds for the target screen to render.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
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
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Failure in screenshot comparison: ", err)
	}
}

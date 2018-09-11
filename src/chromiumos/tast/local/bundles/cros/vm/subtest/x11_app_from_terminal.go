// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package subtest

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// Executes a test X11 application directly from the command line in the terminal
// and verifies that it draws to the screen successfully.
func X11AppFromTerminal(s *testing.State, cont *vm.Container) {
	s.Log("Executing X11 app test from terminal launch")

	// Launch the X11 test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	cmd := cont.Command(s.Context(), "/opt/google/cros-containers/bin/x11_demo", "0x99ee44")
	if err := cmd.Start(); err != nil {
		cmd.DumpLog(s.Context())
		s.Error("Failed launching the x11_demo application")
		return
	}

	const screenshotName = "screenshot_x11_terminal.png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x0100

	expectedColor := screenshot.Color{0x9999, 0xeeee, 0x4444}
	// Allow up to 10 seconds for the target screen to render.
	err := testing.Poll(s.Context(), func(context.Context) error {
		if err := screenshot.Capture(s.Context(), path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			s.Fatal("Failed opening the screenshot image: ", err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			s.Fatal("Failed decoding the screenshot image: ", err)
		}
		color, ratio := screenshot.DominantColor(im)
		if ratio >= 0.5 && screenshot.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		} else {
			return fmt.Errorf("screenshot did not have matching dominant color, expected: "+
				"%v but got: %v at ratio %v", expectedColor, color, ratio)
		}
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	// Terminate the x11 app now so that if there's a failure in the screenshot
	// then we can get its output which may give us useful information about
	// display errors.
	cmd.Kill()
	cmd.Wait()
	if err != nil {
		cmd.DumpLog(s.Context())
		s.Error("Failure in screenshot comparison for X11 app from terminal: ", err)
	}
}

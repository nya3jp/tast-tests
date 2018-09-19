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

// VerifyAppFromTerminal executes a test application directly from the command
// line in the terminal and verifies that it renders the majority of pixels on
// the screen in the specified color.
func VerifyAppFromTerminal(s *testing.State, cont *vm.Container, name,
	command string, expectedColor screenshot.Color) {
	s.Log("Executing test app from terminal launch for ", name)
	ctx := s.Context()
	// Launch the test app which will maximize itself and then use the
	// argument as a solid color to fill as its background.
	commandColor := fmt.Sprintf("0x%02x%02x%02x", expectedColor.R>>8,
		expectedColor.G>>8, expectedColor.B>>8)
	cmd := cont.Command(ctx, command, commandColor)
	if err := cmd.Start(); err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failed launching %v: %v", command, err)
		return
	}

	screenshotName := "screenshot_terminal_" + name + ".png"
	path := filepath.Join(s.OutDir(), screenshotName)

	// Largest differing color known to date, we will be changing this over time
	// based on testing results.
	const maxKnownColorDiff = 0x0100

	// Allow up to 10 seconds for the target screen to render.
	err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := screenshot.Capture(ctx, path); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			s.Fatalf("Failed opening the screenshot image %v: %v", path, err)
		}
		defer f.Close()
		im, err := png.Decode(f)
		if err != nil {
			s.Fatal("Failed decoding the screenshot image %v: %v", path, err)
		}
		color, ratio := screenshot.DominantColor(im)
		if ratio >= 0.5 && screenshot.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		}
		return fmt.Errorf("screenshot did not have matching dominant color, expected "+
			"%v but got %v at ratio %v", expectedColor, color, ratio)
	}, &testing.PollOptions{Timeout: 10 * time.Second})

	// Terminate the app now so that if there's a failure in the
	// screenshot then we can get its output which may give us useful information
	// about display errors.
	cmd.Kill()
	cmd.Wait()
	if err != nil {
		defer cmd.DumpLog(ctx)
		s.Errorf("Failure in screenshot comparison for %v from terminal: %v", name, err)
	}
}

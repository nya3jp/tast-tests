// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package copypaste

import (
	"context"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Mode An enum for different source/destination copy paste protocols
type Mode int

// Wayland Use the wayland copy/paste protocol
const Wayland Mode = iota

// RunTest Run a copy paste test with the supplied parameters
func RunTest(ctx context.Context, s *testing.State, CopyMode Mode, CopyMimeType string, CopyData string, PasteMode Mode, PasteMimeType string, ExpectedPasteData string) {
	pre := s.PreValue().(crostini.PreData)
	cr := pre.Chrome
	cont := pre.Container

	cmd := cont.Command(ctx, "/opt/google/cros-containers/bin/wayland_copy_demo", CopyMimeType, CopyData)
	err := cmd.Start()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to start copy application: ", err)
	}

	screenshotName := "screenshot_copy_demo.png"
	path := filepath.Join(s.OutDir(), screenshotName)

	expectedColor := color.White
	const maxKnownColorDiff = 0x1

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
		if ratio >= 0.5 && colorcmp.ColorsMatch(color, expectedColor, maxKnownColorDiff) {
			return nil
		}
		return errors.Errorf("screenshot did not have matching dominant color, expected %v but got %v at ratio %0.2f",
			colorcmp.ColorStr(expectedColor), colorcmp.ColorStr(color), ratio)

	}, &testing.PollOptions{Timeout: 10 * time.Second})
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Copy application did not appear to start: ", err)
	}

	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	keyboard.Type(ctx, " ")

	err = cmd.Wait()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to wait on copy application: ", err)
	}

	cmd = cont.Command(ctx, "/opt/google/cros-containers/bin/wayland_paste_demo", PasteMimeType)
	output, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run paste application: ", err)
	}

	if string(output) != ExpectedPasteData {
		s.Error("Paste output was \"", string(output), "\", expected \"", ExpectedPasteData, "\"")
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VTSwitch,
		Desc:         "Switch between VT-2 shell and GUI multiple times",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeGraphics",
		Params: []testing.Param{{
			Name:      "smoke",
			ExtraAttr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
			Val:       2,
		}, {
			Name:      "stress",
			ExtraAttr: []string{"group:graphics", "graphics_weekly"},
			Val:       100,
			Timeout:   22 * time.Minute,
		}},
	})
}

const (
	waitTime = 5 * time.Second
)

func inputCheck(ctx context.Context) (*input.KeyboardEventWriter, error) {
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open the keyboard")
	}
	return kb, nil
}

func openVT1(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching to VT1")
	kb, err := inputCheck(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open VT1")
	}
	keyboardKey := "ctrl+alt+back"
	if err = kb.Accel(ctx, keyboardKey); err != nil {
		return errors.Wrapf(err, "failed to press key %q", keyboardKey)
	}
	// Allowing some wait time for switching to happen.
	// TODO(b:198837833): Replace with testing.Poll to query the current vts node.
	if err = testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "error while waiting for switching to VT1")
	}
	return nil
}

func openVT2(ctx context.Context) error {
	testing.ContextLog(ctx, "Switching to VT2")
	kb, err := inputCheck(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to open VT1")
	}
	keyboardKey := "ctrl+alt+refresh"
	if err = kb.Accel(ctx, keyboardKey); err != nil {
		return errors.Wrapf(err, "failed to press key %q", keyboardKey)
	}
	// Allowing some wait time for switching to happen.
	// TODO(b:198837833): Replace with testing.Poll to query the current vts node.
	if err = testing.Sleep(ctx, waitTime); err != nil {
		return errors.Wrap(err, "error while waiting for switching to VT2")
	}
	return nil
}

func takeVTScreenshot(ctx context.Context, s *testing.State, fileName string) {
	if err := screenshot.Capture(ctx, fileName); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}
}

func savePerf(number int, name, unit string, pv *perf.Values) {
	direction := perf.BiggerIsBetter
	if unit == "percent" {
		direction = perf.SmallerIsBetter
	}
	pv.Set(perf.Metric{
		Name:      name,
		Unit:      unit,
		Direction: direction,
	}, float64(number))
}

// isPerceptualDiff opens a terminal and runs perceptualdiff between two images
func isPerceptualDiff(ctx context.Context, s *testing.State, file1, file2 string) bool {

	_, err := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", file1, file2).CombinedOutput()
	status, _ := testexec.ExitCode(err)
	return status == 0
}

// VTSwitch will switch between VT-1 and VT-2 for multiple times.
func VTSwitch(ctx context.Context, s *testing.State) {

	iterations := s.Param().(int)
	s.Logf("No. of iterations: %d", iterations)
	numErrors := 0

	_ = s.FixtValue().(*chrome.Chrome)

	defer func(ctx context.Context) {
		if err := openVT1(ctx); err != nil {
			s.Fatal("Failed to open VT1: ", err)
		}
	}(ctx)

	// Make sure we start in VT1.
	if err := openVT1(ctx); err != nil {
		s.Fatal("Failed to open VT1: ", err)
	}

	// Take VT1 screenshot
	vt1Screenshot := filepath.Join(s.OutDir(), "Initial_VTSwitch_VT1.png")
	takeVTScreenshot(ctx, s, vt1Screenshot)

	// Go to VT2 and take screenshot
	if err := openVT2(ctx); err != nil {
		s.Fatal("Failed to open VT2: ", err)
	}
	vt2Screenshot := filepath.Join(s.OutDir(), "Initial_VTSwitch_VT2.png")
	takeVTScreenshot(ctx, s, vt2Screenshot)

	// Make sure VT1 and VT2 are sufficiently different.
	if isPerceptualDiff(ctx, s, vt1Screenshot, vt2Screenshot) {
		numErrors++
		s.Error("Initial VT1 and VT2 screenshots are perceptually similar")
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	numIdenticalVT1Screenshots := 0
	numIdenticalVT2Screenshots := 0
	maxVT1Difference := 0
	maxVT2Difference := 0

	// Repeatedly switch between VT1 and VT2 images.
	for i := 0; i < iterations; i++ {
		// Go to VT1 and take screenshot.
		if err := openVT1(ctx); err != nil {
			s.Fatal("Failed to open VT1: ", err)
		}
		fileName := fmt.Sprintf("VTSwitch_VT1_%d.png", i)
		currentVT1Screenshot := filepath.Join(s.OutDir(), fileName)
		takeVTScreenshot(ctx, s, currentVT1Screenshot)

		// Check if the current VT1 screenshot is similar to the original VT1 screenshot.

		if !isPerceptualDiff(ctx, s, vt1Screenshot, currentVT1Screenshot) {
			s.Error("Initial VT1 and current VT1 screenshots are perceptually different")
			numErrors++
		} else {
			os.Remove(currentVT1Screenshot)
			numIdenticalVT1Screenshots++
		}

		// Go to VT2 and take screenshot.
		if err := openVT2(ctx); err != nil {
			s.Fatal("Failed to open VT2: ", err)
		}
		fileName = fmt.Sprintf("VTSwitch_VT2_%d.png", i)
		currentVT2Screenshot := filepath.Join(s.OutDir(), fileName)
		takeVTScreenshot(ctx, s, currentVT2Screenshot)

		// Check if the current VT2 screenshot is similar to the original VT2 screenshot.

		if !isPerceptualDiff(ctx, s, vt2Screenshot, currentVT2Screenshot) {
			s.Error("Initial VT2 and current VT2 screenshots are perceptually different")
			numErrors++
		} else {
			os.Remove(currentVT2Screenshot)
			numIdenticalVT2Screenshots++
		}
	}

	savePerf(maxVT1Difference, "percent_VT1_screenshot_max_difference", "percent", pv)
	savePerf(maxVT2Difference, "percent_VT2_screenshot_max_difference", "percent", pv)
	savePerf(numIdenticalVT1Screenshots, "num_identical_vt1_screenshots", "count", pv)
	savePerf(numIdenticalVT2Screenshots, "num_identical_vt2_screenshots", "count", pv)

	if numErrors > 0 {
		s.Fatalf("Failed %d/%d switches", numErrors, iterations)
	}
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VTSwitch,
		Desc:         "Switch between VT-2 shell and GUI multiple times",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
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

var (
	re = regexp.MustCompile((`(\d+) pixels are different`))
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

// isPerceptualDiff opens a terminal and runs perceptualdiff between two images.
func isPerceptualDiff(ctx context.Context, file1, file2 string, diffImages bool) (int, error) {

	numPix := 0
	convErr := error(nil)

	stdout, stderr, err := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", file1, file2).SeparatedOutput()
	parseOut := string(stdout[:])
	parseErr := string(stderr[:])

	// try to find the number of pixels different, if its in the output.
	parsePixels := re.FindStringSubmatch(parseOut)
	if len(parsePixels) > 1 {
		pixels := parsePixels[1]
		numPix, convErr = strconv.Atoi(pixels)
	}

	// if converting from string to int didnt work raise a error.
	if convErr != nil {
		return numPix, errors.Wrap(err, "failed to convert pixels from string to int")
	}

	// if the error code says images are different and they shouldnt be return error.
	if err != nil && !diffImages {
		return numPix, errors.Wrap(err, "images are different")
	}

	// check if there are errors with the command
	if len(parseErr) > 0 {
		return numPix, errors.Wrap(err, "error occured while running perceptualdiff")
	}

	return numPix, nil
}

func max(first, second int) int {
	if first > second {
		return first
	}
	return second
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
	if err := screenshot.Capture(ctx, vt1Screenshot); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	// Go to VT2 and take screenshot
	if err := openVT2(ctx); err != nil {
		s.Fatal("Failed to open VT2: ", err)
	}

	vt2Screenshot := filepath.Join(s.OutDir(), "Initial_VTSwitch_VT2.png")
	if err := screenshot.Capture(ctx, vt2Screenshot); err != nil {
		s.Error("Failed to take screenshot: ", err)
	}

	// Make sure VT1 and VT2 are sufficiently different.
	isSame, ok := isPerceptualDiff(ctx, vt1Screenshot, vt2Screenshot, true)

	if ok != nil {
		s.Fatal("Error occurred while comparing VT1 and VT2 screenshots")
	}
	if isSame == 0 {
		numErrors++
		s.Error("Initial VT1 and VT2 screenshots are perceptually similar")
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	var identicalScreenshotsArr [2]int
	var maxDifferenceArr [2]int

	captureAndCompare := func(vt, id int, original string) {

		fileName := fmt.Sprintf("VTSwitch_VT%d_%d.png", vt, id)
		currtVTScreenshot := filepath.Join(s.OutDir(), fileName)
		if err := screenshot.Capture(ctx, currtVTScreenshot); err != nil {
			s.Error("Failed to take screenshot: ", err)
		}

		numPix, ok := isPerceptualDiff(ctx, original, currtVTScreenshot, false)
		if ok == nil {
			identicalScreenshotsArr[vt-1]++
			return
		}

		if ok != nil && numPix == 0 {
			s.Errorf("Failed to run perceptual diff in iteration %d for VT %d and current VT %d ", id, vt, vt)
			return
		}

		if ok != nil && numPix > 0 {
			s.Errorf("Initial VT %d and current VT %d are different in iteration %d by %d pixels", vt, vt, id, numPix)
			maxDifferenceArr[vt-1] = max(maxDifferenceArr[vt-1], numPix)
			return
		}

	}
	// Repeatedly switch between VT1 and VT2 images.
	for i := 0; i < iterations; i++ {
		if err := openVT1(ctx); err != nil {
			s.Fatalf("Failed to open vt1 at iteration %d", i)
		}
		captureAndCompare(1, i, vt1Screenshot)

		if err := openVT2(ctx); err != nil {
			s.Fatalf("Failed to open vt2 at iteration %d", i)
		}
		captureAndCompare(2, i, vt2Screenshot)
	}

	savePerf(maxDifferenceArr[0], "percent_VT1_screenshot_max_difference", "percent", pv)
	savePerf(maxDifferenceArr[1], "percent_VT2_screenshot_max_difference", "percent", pv)
	savePerf(identicalScreenshotsArr[0], "num_identical_vt1_screenshots", "count", pv)
	savePerf(identicalScreenshotsArr[1], "num_identical_vt2_screenshots", "count", pv)

	if numErrors > 0 {
		s.Fatalf("Failed %d/%d switches", numErrors, iterations)
	}
}

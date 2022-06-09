// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Switch between VT-2 shell and GUI multiple times",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		//TODO(198837833): Remove hwdep.InternalKeyboard() and use argument to frecon to do vt switching instead of typing keys.
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.InternalKeyboard()),
		Fixture:      "chromeGraphics",
		Params: []testing.Param{{
			Name:      "smoke",
			ExtraAttr: []string{"group:mainline", "informational", "group:graphics", "graphics_nightly"},
			Val:       2,
		}, {
			Name:      "stress",
			ExtraAttr: []string{"group:graphics", "graphics_weekly"},
			Val:       25,
			Timeout:   22 * time.Minute,
		}},
	})
}

const (
	waitTime      = 5 * time.Second
	samenessRatio = 0.05
)

var (
	perceptualDiffRe = regexp.MustCompile((`(\d+) pixels are different`))
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
		return errors.Wrap(err, "failed to open VT2")
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

func savePerf(number float64, name, unit string, pv *perf.Values) {
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

// isPerceptuallySame opens a terminal and runs perceptualdiff between two images.
func isPerceptuallySame(ctx context.Context, file1, file2 string, thresholdRatio float64) (bool, float64, error) {

	numPix := 0.0
	convErr := error(nil)
	isSame := false

	fs, err := os.Open(file1)
	defer fs.Close()
	if err != nil {
		return isSame, numPix, errors.Wrap(err, "failed to open vt1 file to compare")
	}

	img, _, err := image.Decode(fs)
	if err != nil {
		return isSame, numPix, errors.Wrap(err, "failed to decode vt1 image")
	}

	imagePixels := img.Bounds().Max.X * img.Bounds().Max.Y
	thresholdPixels := fmt.Sprintf("%f", thresholdRatio*float64(imagePixels))
	stdout, stderr, err := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", thresholdPixels, file1, file2).SeparatedOutput(testexec.DumpLogOnError)
	// If images were different this regex would have some match.
	matched := perceptualDiffRe.FindStringSubmatch(string(stdout))

	// If some error occurred and it was not due to images being different.
	if (err != nil && len(matched) == 0) || len(string(stderr)) > 0 {

		return isSame, numPix, errors.Wrap(err, "error occurred while running perceptual diff")
	}

	// Try to find the number of pixels different, if its in the output.
	if len(matched) > 1 {
		differentPixels := matched[1]
		numPix, convErr = strconv.ParseFloat(differentPixels, 64)
		// If converting from string to int didn't work raise a error.
		if convErr != nil {
			return isSame, numPix, errors.Wrap(err, "failed to convert pixels from string to int")
		}
	}

	pixelDifferenceRatio := (float64(numPix) / float64(imagePixels))
	// At this stage the command has ran successfully and can either be a match or no match.
	isSame = strings.Contains(string(stdout), "PASS") && pixelDifferenceRatio < thresholdRatio
	return isSame, pixelDifferenceRatio, nil
}

func max(first, second float64) float64 {
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
	isSame, initialRatio, err := isPerceptuallySame(ctx, vt1Screenshot, vt2Screenshot, 0.0)
	samenessThreshold := samenessRatio * initialRatio

	s.Logf("The initial samenessThreshold is %f and the initialRatio is %f", samenessThreshold, samenessRatio)

	if err != nil {
		s.Fatal("Error occurred while comparing Initial VT1 and VT2 screenshots")
	}

	if isSame {
		numErrors++
		s.Fatal("Initial VT1 and VT2 screenshots are perceptually similar")
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	var identicalScreenshots [3]int
	var maxDifferenceRatio [3]float64

	captureAndCompare := func(vt, id int, original string) {

		fileName := fmt.Sprintf("VTSwitch_VT%d_%d.png", vt, id)
		currtVTScreenshot := filepath.Join(s.OutDir(), fileName)
		if err := screenshot.Capture(ctx, currtVTScreenshot); err != nil {
			s.Error("Failed to take screenshot: ", err)
		}

		isSame, diffPixelsRatio, err := isPerceptuallySame(ctx, original, currtVTScreenshot, samenessThreshold)

		if err != nil {
			s.Errorf("Perceptual difference failed to run when testing Initial and current VT%d in iteration %d, %d", vt, id, err)
			return
		}
		if isSame {
			identicalScreenshots[vt]++
			err := os.Remove(currtVTScreenshot)
			if err != nil {
				s.Errorf("Error deleting file %s", currtVTScreenshot)
			}
		} else {
			s.Errorf("Failed to switch from VT %d terminals in iteration %d", vt, id)
			maxDifferenceRatio[vt] = max(maxDifferenceRatio[vt], diffPixelsRatio)
		}
		return
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

	savePerf(100.00*maxDifferenceRatio[1], "percent_VT1_screenshot_max_difference", "percent", pv)
	savePerf(100.00*maxDifferenceRatio[2], "percent_VT2_screenshot_max_difference", "percent", pv)
	savePerf(float64(identicalScreenshots[1]), "num_identical_vt1_screenshots", "count", pv)
	savePerf(float64(identicalScreenshots[2]), "num_identical_vt2_screenshots", "count", pv)

	if numErrors > 0 {
		s.Fatalf("Failed %d/%d switches", numErrors, iterations)
	}
}

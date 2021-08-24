// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
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
	waitTime                   = 5 * time.Second
	differencePercentThreshold = 5
	similarityPercentThreshold = 95
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

	takeVTScreenshot := func(fileName string) {
		if err := screenshot.Capture(ctx, fileName); err != nil {
			s.Error("Failed to take screenshot: ", err)
		}
	}

	// Take VT1 screenshot
	vt1Screenshot := filepath.Join(s.OutDir(), "Initial_VTSwitch_VT1.png")
	takeVTScreenshot(vt1Screenshot)

	// Go to VT2 and take screenshot
	if err := openVT2(ctx); err != nil {
		s.Fatal("Failed to open VT2: ", err)
	}
	vt2Screenshot := filepath.Join(s.OutDir(), "Initial_VTSwitch_VT2.png")
	takeVTScreenshot(vt2Screenshot)

	loadImage := func(filename string) (image.Image, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, err
		}
		return img, nil
	}

	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}

	difference := func(a, b uint32) int64 {
		if a > b {
			return int64(a - b)
		}
		return int64(b - a)
	}

	getPercentDifference := func(file1, file2 string) float64 {
		vtFile1, err := loadImage(file1)
		if err != nil {
			s.Fatalf("Failed to load the image %q: %v", file1, err)
		}
		vtFile2, err := loadImage(file2)
		if err != nil {
			s.Fatalf("Failed to load the image %q: %v", file2, err)
		}
		b := vtFile1.Bounds()
		var sum int64
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r1, g1, b1, _ := vtFile1.At(x, y).RGBA()
				r2, g2, b2, _ := vtFile2.At(x, y).RGBA()
				sum += difference(r1, r2)
				sum += difference(g1, g2)
				sum += difference(b1, b2)
			}
		}
		nPixels := (b.Max.X - b.Min.X) * (b.Max.Y - b.Min.Y)
		return float64(sum*100) / (float64(nPixels) * 0xffff * 3)
	}

	// Make sure VT1 and VT2 are sufficiently different.
	initialDiff := int(getPercentDifference(vt1Screenshot, vt2Screenshot))
	if initialDiff < differencePercentThreshold {
		numErrors++
		s.Errorf("Initial VT1 and VT2 screenshots differ by %d %%", initialDiff)
	}

	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			s.Error("Failed to save perf data: ", err)
		}
	}()

	numIdenticalVT1Screenshots := 0
	numIdenticalVT2Screenshots := 0
	maxVT1DifferencePercent := 0
	maxVT2DifferencePercent := 0

	// Repeatedly switch between VT1 and VT2.
	for i := 0; i < iterations; i++ {
		// Go to VT1 and take screenshot.
		if err := openVT1(ctx); err != nil {
			s.Fatal("Failed to open VT1: ", err)
		}
		fileName := fmt.Sprintf("VTSwitch_VT1_%d.png", i)
		currentVT1Screenshot := filepath.Join(s.OutDir(), fileName)
		takeVTScreenshot(currentVT1Screenshot)

		// Check if the current VT1 screenshot is similar to the original VT1 screenshot.
		diff := int(getPercentDifference(vt1Screenshot, currentVT1Screenshot))
		if (100 - diff) <= similarityPercentThreshold {
			s.Errorf("Initial VT1 and current VT1 screenshots differ by %d %% in %d iteration", diff, i)
			maxVT1DifferencePercent = max(diff, maxVT1DifferencePercent)
			numErrors++
		} else {
			numIdenticalVT1Screenshots++
		}

		// Go to VT2 and take screenshot.
		if err := openVT2(ctx); err != nil {
			s.Fatal("Failed to open VT2: ", err)
		}
		fileName = fmt.Sprintf("VTSwitch_VT2_%d.png", i)
		currentVT2Screenshot := filepath.Join(s.OutDir(), fileName)
		takeVTScreenshot(currentVT2Screenshot)

		// Check if the current VT2 screenshot is similar to the original VT2 screenshot.
		diff = int(getPercentDifference(vt2Screenshot, currentVT2Screenshot))
		if (100 - diff) <= similarityPercentThreshold {
			s.Errorf("Initial VT2 and current VT2 screenshots differ by %d %% in %d iteration", diff, i)
			maxVT2DifferencePercent = max(diff, maxVT2DifferencePercent)
			numErrors++
		} else {
			numIdenticalVT2Screenshots++
		}
	}

	savePerf := func(number int, name, unit string, pv *perf.Values) {
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

	savePerf(maxVT1DifferencePercent, "percent_VT1_screenshot_max_difference", "percent", pv)
	savePerf(maxVT2DifferencePercent, "percent_VT2_screenshot_max_difference", "percent", pv)
	savePerf(numIdenticalVT1Screenshots, "num_identical_vt1_screenshots", "count", pv)
	savePerf(numIdenticalVT2Screenshots, "num_identical_vt2_screenshots", "count", pv)

	if numErrors > 0 {
		s.Fatalf("Failed %d/%d switches", numErrors, iterations)
	}
}

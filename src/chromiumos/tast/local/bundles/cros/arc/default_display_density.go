// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"math"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DefaultDisplayDensity,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that the default density is calculated correctly for various boards",
		Contacts:     []string{"prabirmsp@chromium.org", "arc-framework+tast@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_vm", "chrome"},
		Timeout:      4 * time.Minute,
		Fixture:      "arcBooted",
	})
}

func DefaultDisplayDensity(ctx context.Context, s *testing.State) {
	const (
		// The ratio between Chrome's scale and Android's scale.
		ChromeScaleToAndroidScaleRatio = 0.75
		// The Uniform Scale Factor (USF) that is used by default on ARC++ R and later.
		UniformScaleFactor = 1.20
	)

	cr := s.FixtValue().(*arc.PreData).Chrome
	a := s.FixtValue().(*arc.PreData).ARC

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	var defaultDeviceScaleFactor float64
	dispInfo, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		// An internal display was not found. Use a device scale factor of 1.0.
		defaultDeviceScaleFactor = 1.0
	} else {
		// An internal display is present. Assume that it is using the default zoom factor on boot.
		if dispInfo.DisplayZoomFactor != 1.0 {
			s.Fatalf("The internal display is configured with a non-default zoom factor %f", dispInfo.DisplayZoomFactor)
		}

		defaultDeviceScaleFactor, err = dispInfo.GetEffectiveDeviceScaleFactor()
		if err != nil {
			s.Fatal("Failed to get the effective device scale factor: ", err)
		}
	}

	// The Android density DPI is calculated from Chrome's device scale factor. For low-density devices, we ensure that
	// the chosen density does not fall below DefaultDensityDPI, because using lower density values can cause performance
	// and compatibility issues.
	calculatedDPI := math.Max(1.0, defaultDeviceScaleFactor*ChromeScaleToAndroidScaleRatio*UniformScaleFactor) * arc.DefaultDensityDPI
	expectedDPI := closestCTSApprovedDensityDPI(calculatedDPI)

	d, err := arc.NewDisplay(a, arc.DefaultDisplayID)
	if err != nil {
		s.Fatal("Failed to create new arc display: ", err)
	}

	dpi, err := d.OverrideDensityDPI(ctx)
	if err != nil {
		s.Fatal("Failed to get override display density: ", err)
	}

	if expectedDPI != dpi {
		s.Fatalf("The default density was not set correctly: got: %d want: %d", dpi, expectedDPI)
	}
}

// closestCTSApprovedDensityDPI returns the density DPI that is approved by the Android CDD that is closest to
// the provided density. Android CDD requires that devices must use one of the approved density DPI by default.
// If provided density falls exactly in the middle of two values, we pick the larger value.
func closestCTSApprovedDensityDPI(dpi float64) (bestDPI int) {
	// These are the list of approved density values reported in the Android CDD.
	// Source: https://source.android.com/compatibility/android-cdd#7_1_1_3_screen_density
	var ctsDensityBuckets = [...]int{120, 140, 160, 180, 200, 213, 220, 240, 260, 280, 300, 320, 340, 360, 400, 420, 480, 560, 640}

	minDiff := math.MaxFloat64
	for _, d := range ctsDensityBuckets {
		diff := math.Abs(float64(d) - dpi)
		if diff > minDiff {
			break
		}
		minDiff = diff
		bestDPI = d
	}
	return bestDPI
}

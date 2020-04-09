// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerAppDensity,
		Desc:         "Checks that density can be changed with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const perAppDensityApk = "ArcPerAppDensityTest.apk"

func PerAppDensity(ctx context.Context, s *testing.State) {
	const (
		packageName = "org.chromium.arc.testapp.perappdensitytest"
		// The following scale factors have been taken from TaskRecordArc.
		increasedSF = 1.1
		decreasedSF = 0.9
		// Defined in XML files in vendor/google_arc/packages/developments/ArcPerAppDensityTest/res/layout.
		squareSidePx = 100
	)

	perappdensity.RunTest(ctx, s, perAppDensityApk, packageName, []string{".ViewActivity", ".SurfaceViewActivity"}, func(ctx context.Context, cr *chrome.Chrome, a *arc.ARC, ew *input.KeyboardEventWriter, activityName string, displayDensity float64) error {
		expectedInitialPixelCount := (displayDensity * squareSidePx) * (displayDensity * squareSidePx)

		if err := perappdensity.CheckBlackPixels(ctx, cr, int(expectedInitialPixelCount)); err != nil {
			s.Fatal("Failed to check initial state: ", err)
		}

		for _, test := range []perappdensity.DensityChange{
			{
				"increase",
				"ctrl+=",
				expectedInitialPixelCount * float64(increasedSF) * float64(increasedSF),
			},
			{
				"reset",
				"ctrl+0",
				expectedInitialPixelCount,
			},
			{
				"decrease",
				"ctrl+-",
				expectedInitialPixelCount * float64(decreasedSF) * float64(decreasedSF),
			},
		} {
			// Return density to initial state.
			defer func() {
				if err := perappdensity.PerformAndConfirmDensityChange(ctx, cr, ew, a, "reset", "ctrl+0", int(expectedInitialPixelCount)); err != nil {
					s.Fatalf("Error with performing %s: %s", test.Name, err)
				}
			}()
			if err := perappdensity.PerformAndConfirmDensityChange(ctx, cr, ew, a, test.Name, test.KeySequence, int(test.BlackPixelCount)); err != nil {
				s.Fatalf("Error with performing %s: %s", test.Name, err)
			}
		}
		return nil
	})
}
